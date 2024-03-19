<a href="https://opensource.newrelic.com/oss-category/#new-relic-experimental"><picture><source media="(prefers-color-scheme: dark)" srcset="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/dark/Experimental.png"><source media="(prefers-color-scheme: light)" srcset="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Experimental.png"><img alt="New Relic Open Source experimental project banner." src="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Experimental.png"></picture></a>
# nri-mark-mobile-mobile-deployment
This application monitors the Mobile Applications in a New Relic Account for new releases and adds [Change Tracking](https://docs.newrelic.com/docs/change-tracking/change-tracking-introduction/) descriptors to the Mobile Application Entity.

The application discovers Mobile Applications in the account by searching for unique Mobile Event `entityGuid`s. The application then searches for new `appVersions` by `entityGuid`

## Caveats
There are several restrictions that apply to this process, all can be overridden _at your own risk_:
- New Change Tracking descriptors must have a timestamp +/- 24 hours from the time of detection
  - This means that old releases are unseen unless you modify `-discoverVersionsSince`
  - Any release older than 24 hours will have its `timestamp` set to the current time- probably not what you want
- Mobile Applications which have had no traffic in the past 3 months are removed from NRDB however the Mobile Events remain and are orphaned. Setting `-discoverAppsSince` beyond 3 months will trip on this and you'll see errors from 
  `setDeployment` in the log as there is no Entity to Change Track against. The errors are harmless.

## Installation
- Install Go 1.22 on the target system
- Clone this repo
- `cd` into the cloned repo 
- run `go mod tidy` to install the dependencies
- run `go build cmd/nri-mark-mobile-deployment/nri-mark-mobile-deployment.go` to build the application

### Notes
- The application does not contain a scheduler, it runs once and exits. Our suggestion is to run it under `cron` or something similar at least every 12 hours

## Usage
Under normal circumstances usage would look like this
```bash
./nri-mark-mobile-deployment -apiKey="<YOUR_NEWRELIC_USER_KEY>" -accountId=<YOUR_NEWRELIC_ACCOUNT_ID>
```

An example crontab entry to run the application at 0 minutes past every 12th hour
```cronexp
0 */12 * * *  <full_path_to>/nri-mark-mobile-deployment -apiKey="<YOUR_API_KEY>" -accountId=<YOUR_ACCOUNT_ID>
```

All command line parameters:
```bash
./nri-mark-mobile-deployment -help
Usage of ./nri-mark-mobile-deployment:
  -accountId int
    	New Relic account id
  -apiKey string
    	New Relic User Key
  -appConfigFile string
    	JSON file containing this apps's configuration (default "apps.json")
  -customAttributes string
    	Custom attributes as JSON object (default "{}")
  -discoverAppsSince string
    	Valid NRQL since clause that determines how far back to search for application entityGuids (default "3 months ago")
  -discoverOnly
    	Enable to generate config file only
  -discoverVersionsSince string
    	Valid NRQL since clause that determines how far back to search for new releases (default "24 hours ago")
  -logLevel string
    	Logging level: info | debug | warn | error (default "INFO")
```

| Param                  | Description                                         | Default                        | Required | Notes                                                                            |
|:-----------------------|:----------------------------------------------------|:-------------------------------|:--------:|:---------------------------------------------------------------------------------|
| -accountId             | New Relic Account Id                                |                                |    X     |                                                                                  |
| -apiKey                | New Relic User Key                                  |                                |    X     | https://docs.newrelic.com/docs/apis/intro-apis/new-relic-api-keys/#overview-keys |
| -appConfigFile         | State file                                          | `./apps.json                   |          |                                                                                  |
| -customAttributes      | Custom attributes attached to Change Descriptor     | {}                             |          | JSON Object (key/value) string                                                   |
| -discoverAppsSince     | NRQL `since` clause for App search                  | `3 months ago`                 |          | https://docs.newrelic.com/docs/nrql/nrql-syntax-clauses-functions/#sel-since     |
| -discoverOnly          | Discover Apps and Versions, write config file, exit | `false`                        |          |                                                                                  |
| -discoverVersionsSince | NRQL `since` clause for Release search              | `24 hours ago`                 |          | https://docs.newrelic.com/docs/nrql/nrql-syntax-clauses-functions/#sel-since     |
| -logLevel              | Logging level                                       | INFO \| ERROR \| WARN \| DEBUG |   INFO   |                                                                                  | |

### appConfigFile
The application maintains state in `appConfigFile` and requires read/write permission here. There is no user maintained state in this file, it is a JSON representation of what the application knows with respect to Mobile Applications 
and their released versions. The file is read on startup and written on exit to maintain the current state.

### discoverAppsSince
Mobile Applications which have had no traffic in the past 3 months are removed from NRDB however the Mobile Events remain and are orphaned. Setting `-discoverAppsSince` beyond 3 months will trip on this and you'll see errors from
  `setDeployment` in the log as there is no Entity to Change Track against. The errors are harmless.

### discoverVersionsSince
New Change Tracking descriptors must have a timestamp +/- 24 hours from the time of _detection_
  - This means that old releases are unseen unless you modify `-discoverVersionsSince`
  - Any release older than 24 hours will have its `timestamp` set to the current time- probably not what you want

## Support

New Relic hosts and moderates an online forum where customers can interact with New Relic employees as well as other customers to get help and share best practices. Like all official New Relic open source projects, there's a related Community topic in the New Relic Explorers Hub. You can find this project's topic/threads here:

>Add the url for the support thread here

## Contributing
We encourage your contributions to improve `nri-mark-mobile-deployment`! Keep in mind when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project.
If you have any questions, or to execute our corporate CLA, required if your contribution is on behalf of a company,  please drop us an email at opensource@newrelic.com.

**A note about vulnerabilities**

As noted in our [security policy](../../security/policy), New Relic is committed to the privacy and security of our customers and their data. We believe that providing coordinated disclosure by security researchers and engaging with the security community are important means to achieve our security goals.

If you believe you have found a security vulnerability in this project or any of New Relic's products or websites, we welcome and greatly appreciate you reporting it to New Relic through [HackerOne](https://hackerone.com/newrelic).

## License
`nri-mark-mobile-deployment` is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.