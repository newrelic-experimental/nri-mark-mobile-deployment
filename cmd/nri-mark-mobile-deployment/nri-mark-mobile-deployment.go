package main

import (
   "encoding/json"
   "flag"
   "fmt"
   "github.com/newrelic/newrelic-client-go/v2/pkg/changetracking"
   "github.com/newrelic/newrelic-client-go/v2/pkg/common"
   "github.com/newrelic/newrelic-client-go/v2/pkg/config"
   "github.com/newrelic/newrelic-client-go/v2/pkg/nrdb"
   "github.com/newrelic/newrelic-client-go/v2/pkg/nrtime"
   "io"
   "log/slog"
   "os"
   "strings"
   "time"
)

type Apps struct {
   Apps map[string]*App `json:"apps"`
}

type App struct {
   EntityGuid string              `json:"entityGuid"`
   Versions   map[string]*Version `json:"versions"`
}

type Version struct {
   AppBuild     string `json:"appBuild"`
   AppId        int    `json:"appId"`
   AppName      string `json:"appName"`
   AppVersion   string `json:"appVersion"`
   AppVersionId int    `json:"appVersionId"`
   EntityGuid   string `json:"entityGuid"`
   Timestamp    int    `json:"timestamp"`
}

func (v Version) getTimestamp() int64 {
   result := int64(v.Timestamp)
   timeNow := time.Now().Add(time.Hour * -24).UnixMilli()
   if result < timeNow {
      slog.Warn("getTimestamp: timestamp is more than 24 hours old, using current time", "AppName", v.AppName, "Version", v.AppVersion, "Timestamp", v.Timestamp, "guid", v.EntityGuid)
      result = time.Now().UnixMilli()
   }
   slog.Debug("getTimestamp: ", "result", result)
   return result
}

var accountID = flag.Int("accountId", 0, "New Relic account id")
var apiKey = flag.String("apiKey", "", "New Relic User Key")
var appConfigFile = flag.String("appConfigFile", "apps.json", "JSON file containing this apps's configuration")
var customAttributes = flag.String("customAttributes", `{}`, "Custom attributes as JSON object")
var discoverAppsSince = flag.String("discoverAppsSince", "3 months ago", "Valid NRQL since clause that determines how far back to search for application entityGuids")
var discoverVersionsSince = flag.String("discoverVersionsSince", "24 hours ago", "Valid NRQL since clause that determines how far back to search for new releases")
var discoverOnly = flag.Bool("discoverOnly", false, "Enable to generate config file only")
var logLevel = flag.String("logLevel", "INFO", "Logging level: info | debug | warn | error")

func main() {
   // Config
   flag.Parse()
   switch strings.ToUpper(*logLevel) {
   case "INFO":
      slog.SetLogLoggerLevel(slog.LevelInfo)
   case "DEBUG":
      slog.SetLogLoggerLevel(slog.LevelDebug)
   case "WARN":
      slog.SetLogLoggerLevel(slog.LevelWarn)
   case "ERROR":
      slog.SetLogLoggerLevel(slog.LevelError)
   default:
      slog.SetLogLoggerLevel(slog.LevelInfo)
   }
   if *apiKey == "" {
      slog.Error("apiKey is a required parameter")
      os.Exit(1)
   }
   if *accountID == 0 {
      slog.Error("accountId is a required parameter")
      os.Exit(1)
   }

   apps := loadSavedApps()
   // Query for apps since discoverAppsSince
   apps.update()

   for _, app := range apps.Apps {
      // Query for versions since discoverVersionsSince
      currentVersions, err := app.queryVersions()
      if err != nil {
         slog.Error("Fatal error querying Mobile Events", "error", err)
         os.Exit(1)
      }

      // Figure what's a new version
      versionDelta := app.versionDiff(currentVersions)
      if !*discoverOnly {
         // Mark ONLY a version
         versionDelta.setDeployment()
      }

      // Update the app with the new versions
      app.update(versionDelta)
   }
   // Save the entire structure away
   apps.save()
}

func (a *Apps) save() {
   slog.Debug("apps.save", "apps", a)
   // This will truncate the file if it exists, which is what we want
   jsonFile, err := os.Create(*appConfigFile)
   if err != nil {
      slog.Error("Fatal error updating", "appConfigFile", appConfigFile, "error", err)
      os.Exit(1)
   }
   defer jsonFile.Close()

   buffer, err := json.MarshalIndent(a, "", "  ")
   if err != nil {
      slog.Error("Fatal error marshaling apps", "error", err)
      os.Exit(1)
   }

   _, err = jsonFile.Write(buffer)
   if err != nil {
      slog.Error("Fatal error writing app config file", "error", err)
   }
}

// Look for and save any newly reporting apps
func (a *Apps) update() {
   discoveredApps := discoverMobileApps()
   for guid, app := range discoveredApps.Apps {
      _, ok := a.Apps[guid]
      if !ok {
         a.Apps[guid] = app
      }
   }
}

// Query NRDB via NRQL looking for new versions
func (a *App) queryVersions() (*App, error) {
   results := &App{
      Versions:   make(map[string]*Version),
      EntityGuid: a.EntityGuid,
   }

   queryString := fmt.Sprintf("select latest(timestamp), latest(appName), latest(appBuild), latest(appId), latest(appVersionId) from Mobile where entityGuid = '%s' since %s facet appVersion", a.EntityGuid, *discoverVersionsSince)
   slog.Debug("queryVersions", "queryString", queryString)

   m := runQuery(queryString)
   for _, r := range m {
      e := &Version{}
      e.Timestamp = int(r["latest.timestamp"].(float64))
      e.AppVersionId = int(r["latest.appVersionId"].(float64))
      e.AppId = int(r["latest.appId"].(float64))
      e.AppBuild = r["latest.appBuild"].(string)
      e.AppName = r["latest.appName"].(string)
      e.AppVersion = r["appVersion"].(string)
      e.EntityGuid = a.EntityGuid
      results.Versions[e.AppVersion] = e
   }
   slog.Debug("queryVersions: ", "results", results, "map", results.Versions)
   return results, nil
}

func (a *App) setDeployment() {
   if len(a.Versions) <= 0 {
      slog.Info("setDeployment: no new versions", "guid", a.EntityGuid)
      return
   }

   slog.Debug("setDeployment: enter", "App", a, "map", a.Versions)
   cfg := config.New()
   cfg.PersonalAPIKey = *apiKey

   client := changetracking.New(cfg)

   for _, event := range a.Versions {

      attrs := make(map[string]interface{})
      err := json.Unmarshal([]byte(*customAttributes), &attrs)
      if err != nil {
         slog.Error("setDeployment: Fatal error unmarshaling custom attributes", "error", err)
         os.Exit(1)
      }
      attrs["AppBuild"] = event.AppBuild
      attrs["AppName"] = event.AppName
      attrs["AppId"] = event.AppId
      attrs["AppVersionId"] = event.AppVersionId

      input := changetracking.ChangeTrackingDeploymentInput{
         CustomAttributes: changetracking.ChangeTrackingRawCustomAttributesMap(attrs),
         DeploymentType:   changetracking.ChangeTrackingDeploymentTypeTypes.BASIC,
         Description:      "Automated deployment marker",
         EntityGUID:       common.EntityGUID(a.EntityGuid),
         GroupId:          "deployment",
         Timestamp:        nrtime.EpochMilliseconds(time.UnixMilli(int64(event.getTimestamp()))),
         User:             "nri-mark-deployment",
         Version:          event.AppVersion,
      }

      changeTrackingDeployment, err := client.ChangeTrackingCreateDeployment(
         changetracking.ChangeTrackingDataHandlingRules{ValidationFlags: []changetracking.ChangeTrackingValidationFlag{changetracking.ChangeTrackingValidationFlagTypes.FAIL_ON_FIELD_LENGTH}},
         input,
      )
      if err != nil {
         slog.Error("setDeployment: error creating deployment", "error", err)
         // TODO Detect the error that tells us the Application has aged out (no activity in the past 3 months) and remove the app from the config
         // Skip further processing on this Application
         continue
      }
      slog.Info("setDeployment", "application", event.AppName, "version", event.AppVersion, "changeTrackingDeployment.EntityGuid", changeTrackingDeployment.EntityGUID)
   }
   slog.Debug("setDeployment: exit")
}

// Update an App with new Versions
func (a *App) update(delta *App) {
   for version, event := range delta.Versions {
      a.Versions[version] = event
   }
}

// Figure-out what's new so we don't apply duplicate markers
func (a *App) versionDiff(versions *App) *App {
   result := &App{EntityGuid: a.EntityGuid, Versions: make(map[string]*Version)}
   for guid, version := range versions.Versions {
      _, ok := a.Versions[guid]
      if !ok {
         result.Versions[guid] = version
      }
   }
   slog.Debug("versionDiff", "result", result, "map", result.Versions)
   return result
}

// Find new Mobile Apps
func discoverMobileApps() *Apps {
   result := &Apps{
      Apps: make(map[string]*App),
   }
   query := fmt.Sprintf(`select uniques(entityGuid) from Mobile since %s`, *discoverAppsSince)
   response := runQuery(query)
   if len(response) == 1 {
      guids, ok := response[0]["uniques.entityGuid"]
      if !ok {
         slog.Error("Fatal error, no entityGuids found in Mobile Events", "accountId", accountID)
         os.Exit(1)
      }
      for _, guid := range guids.([]interface{}) {
         result.Apps[guid.(string)] = &App{EntityGuid: guid.(string), Versions: make(map[string]*Version)}
      }

      slog.Debug("discoverMobileApps", "guids", guids)
      slog.Debug("discoverMobileApps", "response", response)
   }
   return result
}

func loadSavedApps() *Apps {
   var results = Apps{
      Apps: make(map[string]*App),
   }

   jsonFile, err := os.Open(*appConfigFile)
   if err != nil {
      slog.Warn("loadSavedApps: no previous version.json, starting from scratch")
      return discoverMobileApps()
   }
   defer jsonFile.Close()

   byteValue, _ := io.ReadAll(jsonFile)

   err = json.Unmarshal(byteValue, &results)
   if err != nil {
      slog.Error("Fatal error loadSavedApps: unmarshal error: ", "error", err)
      os.Exit(1)
   }
   slog.Debug("loadSavedApps: ", "results", results)
   return &results
}

func runQuery(s string) []nrdb.NRDBResult {
   cfg := config.New()
   cfg.PersonalAPIKey = *apiKey

   client := nrdb.New(cfg)
   query := nrdb.NRQL(s)

   resp, err := client.Query(*accountID, query)
   if err != nil {
      slog.Error("Fatal error running NRQL query: ", "query", query, "error", err)
      os.Exit(1)
   }
   return resp.Results
}
