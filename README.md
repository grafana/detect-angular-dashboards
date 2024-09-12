# detect-angular-dashboards

Detect dashboards depending on Angular data source and panel plugins on a running Grafana instance using the Grafana API.

Compatible with Grafana OSS, Grafana Enterprise, On-Prem and Grafana Cloud.

[Read more about Angular support deprecation in our documentation](https://grafana.com/docs/grafana/latest/developers/angular_deprecation/).

## Configuration

The program requires a service account and API key to work properly.

Configuration changes slightly depending on your version of Grafana.

We recommend using Grafana >= 10.1.0, otherwise Angular private plugins (that are not in Grafana's catalog) won't be detected.

### Grafana >= 10.1.0

Create a service account with `Viewer` role.

Then, create a service account token for the newly created service account and set it to the `GRAFANA_TOKEN` env var.

### Grafana < 10.1.0

> Warning, Angular private plugins won't be detected from the scan when using Grafana <= 10.1.0.

Create a service account, with `Plugins / Plugin Maintainer` permissions (or "Admin" if using OSS without RBAC).

The reason behind admin rights is that the plugins endpoint returns all plugins only if the token can view and install plugins.

Then, create a service account token for the newly created service account and set it to the `GRAFANA_TOKEN` env var.

## Usage
The detect-angular-dashboards binary supports two modes of operation. A CLI mode which can be used on demand, as well as a server mode which periodically quries Grafana for the current set of dashboards and generates a JSON response on the `/detections` endpoint with a list of dashboards that were detected to be using Angular. This endpoint can be linked directly with Grafana by leveraging the [Infinity Datasource](https://grafana.com/grafana/plugins/yesoreyeram-infinity-datasource/). 

### Server Mode
> Pass flag `-server` to run the program in server mode. Value must be a valid listen address ex. "0.0.0.0:8080".
> Pass optional flag `-max-concurrency` to the program to limit the max concurrency when downloading dashboards from Grafana, otherwise default value is used. 
> Pass optional flag `-interval` to the program to set the detection refresh interval when running in server mode, otherwise default value is used. 

```bash
GRAFANA_TOKEN=glsa_aaaaaaaaaaa ./detect-angular-dashboards -server "0.0.0.0:8080" -max-concurrency=10 http://my-grafana.example.com/api
INFO: 2024/09/11 16:59:04 Running detection every 5m0s
INFO: 2024/09/11 16:59:04 Detecting Angular dashboards
INFO: 2024/09/11 16:59:04 Listening on 0.0.0.0:8080
INFO: 2024/09/11 16:59:34 Updating Output Data
INFO: 2024/09/11 16:59:34 Updating readiness probe to ready
```

### CLI Mode - Readable output

```bash
GRAFANA_TOKEN=glsa_aaaaaaaaaaa ./detect-angular-dashboards http://my-grafana.example.com/api
2023/08/17 11:17:12 Detecting Angular dashboards for "http://my-grafana.example.com/api"
2023/08/17 11:17:13 Found dashboard with Angular plugins "Angular" "http://my-grafana.example.com/d/ef5e2c21-88aa-4619-a5db-786cc1dd37a9/angular":
2023/08/17 11:17:13 Found angular panel "Panel two" ("akumuli-datasource")
2023/08/17 11:17:13 Found panel with angular data source "Angular" ("grafana-worldmap-panel")
2023/08/17 11:17:13 Found dashboard with Angular plugins "Datasource tests - Elasticsearch v7" "http://my-grafana.example.com/d/Y-RvmuRWk/datasource-tests-elasticsearch-v7":
2023/08/17 11:17:13 Found panel with angular data source "World map panel" ("grafana-worldmap-panel")
2023/08/17 11:17:14 Found dashboard with Angular plugins "New dashboard" "http://my-grafana.example.com/d/e10a098c-ad80-4d3c-b979-c39a4ce41183/new-dashboard":
2023/08/17 11:17:14 Found angular panel "My panel" ("akumuli-datasource")
```

### CLI Mode - JSON output

> Pass flag -j to the program to output in JSON format to stdout. All other messages will be sent to stderr.
> The example below will produce a valid "output.json" file that can be used with other tools.

```bash
GRAFANA_TOKEN=glsa_aaaaaaaaaaa ./detect-angular-dashboards -j http://my-grafana.example.com/api | tee output.json
2023/08/17 11:25:54 Detecting Angular dashboards for "http://my-grafana.example.com/api"
[
  {
    "Detections": [
      {
        "PluginID": "akumuli-datasource",
        "DetectionType": "datasource",
        "Title": "Panel two"
      },
      {
        "PluginID": "grafana-worldmap-panel",
        "DetectionType": "panel",
        "Title": "Angular"
      }
    ],
    "URL": "http://my-grafana.example.com/d/ef5e2c21-88aa-4619-a5db-786cc1dd37a9/angular",
    "Title": "Angular",
    "Folder": "Angular deprecation",
    "UpdatedBy": "admin",
    "CreatedBy": "admin",
    "Created": "2024-02-22T14:08:06+01:00",
    "Updated": "2024-02-22T14:08:06+01:00"
  },
  {
    "Detections": [
      {
        "PluginID": "grafana-worldmap-panel",
        "DetectionType": "panel",
        "Title": "World map panel"
      }
    ],
    "URL": "http://my-grafana.example.com/d/Y-RvmuRWk/datasource-tests-elasticsearch-v7",
    "Title": "Datasource tests - Elasticsearch v7",
    "Folder": "Angular deprecation",
    "UpdatedBy": "admin",
    "CreatedBy": "admin",
    "Created": "2024-02-22T14:08:06+01:00",
    "Updated": "2024-02-22T14:08:06+01:00"
  },
  {
    "Detections": [
      {
        "PluginID": "akumuli-datasource",
        "DetectionType": "datasource",
        "Title": "My panel"
      }
    ],
    "URL": "http://my-grafana.example.com/d/e10a098c-ad80-4d3c-b979-c39a4ce41183/new-dashboard",
    "Title": "New dashboard",
    "Folder": "Angular deprecation",
    "UpdatedBy": "admin",
    "CreatedBy": "admin",
    "Created": "2024-02-22T14:08:06+01:00",
    "Updated": "2024-02-22T14:08:06+01:00"
  }
]
```

### Using with jq

You can use tools such as `jq` combined with JSON output (`-j`) to get some useful information, such as:

#### Number of affected dashboards

```bash
./detect-angular-dashboards -j | jq '. | length'
36
```

#### Affected dashboard URLs

```bash
./detect-angular-dashboards -j | jq -r '.[].URL'
http://127.0.0.1:3000/d/e1bd7dd5-2ee2-4e47-8e16-4c43c7c12277/a-dashboard-that-contains-some-angular-plugins
http://127.0.0.1:3000/d/7MeksYbmk/alerting-with-testdata
http://127.0.0.1:3000/d/cf2efc3b-1990-4855-9977-32a55fb27452/an-old-dashboard
```

#### Used Angular plugin ids

```bash
./detect-angular-dashboards -j | jq -r ".[].Detections[].PluginID" | sort | uniq

akumuli-datasource
grafana-polystat-panel
grafana-worldmap-panel
graph
table-old
```

#### Used Angular plugin ids and number of occurrences

```bash
./detect-angular-dashboards -j | jq -r ".[].Detections[].PluginID" | sort | uniq -c | sort -nr
    222 graph
      7 table-old
      4 grafana-polystat-panel
      2 grafana-worldmap-panel
      1 akumuli-datasource
```

### Running against multiple organizations

If you have multiple organizations on your Grafana instance, you have to run the tool against each organization.
To do so, you first have to create a service account and token for each organization, and then
run the program with each service account token. The Grafana URL is the same for every organization.

### Using pre-built binaries

You can download pre-built binaries from the [releases](https://github.com/grafana/detect-angular-dashboards/releases) section.

Then, run the program. Replace `http://127.0.0.1:3000` with the URL of your Grafana instance.

```bash
GRAFANA_TOKEN=abcd ./detect-angular-dashboards http://127.0.0.1:3000/api
```

### Building from source

You need to install [Go](https://go.dev) and [Mage](https://magefile.org/).

Then, clone the repository, build and run the program. Replace `http://127.0.0.1:3000` with the URL of your Grafana instance.

```bash
mage build:current
GRAFANA_TOKEN=abcd ./dist/linux_amd64/detect-angular-dashboards http://127.0.0.1:3000/api
```

### Docker image

Clone the repository and build the Docker image. Replace `http://127.0.0.1:3000` with the URL of your Grafana instance.

```bash
docker build -t detect-angular-dashboards .
docker run --rm -it -e GRAFANA_TOKEN=abcd detect-angular-dashboards http://172.17.0.1:3000/api
```

## LICENSE

Apache License 2.0
