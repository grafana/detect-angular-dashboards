# detect-angular-dashboards

Detect dashboards depending on Angular data source and panel plugins on a running Grafana instance using the Grafana API.

Compatible with Grafana OSS, Grafana Enterprise, On-Prem and Grafana Cloud.

[Read more about Angular support deprecation in our documentation](https://grafana.com/docs/grafana/latest/developers/angular_deprecation/).

## Configuration

The program requires a service account and API key to work properly.

Configuration changes slightly depending on your version of Grafana.

We recommend using Grafana >= 10.1.0, otherwise Angular private plugins (that are not in Grafana's catalog) will be ignored.

### Grafana >= 10.1.0

Create a service account with `Viewer` role.

Then, create a service account token for the newly created service account and set it to the `GRAFANA_TOKEN` env var.

### Grafana < 10.1.0

> Warning, Angular private plugins will be ignored from the scan when using Grafana <= 10.1.0.

Create a service account, with `Plugins / Plugin Maintainer` permissions (or "Admin" if using OSS without RBAC).

The reason behind admin rights is that the plugins endpoint returns all plugins only if the token can view and install plugins.

Then, create a service account token for the newly created service account and set it to the `GRAFANA_TOKEN` env var.



## Usage

### Readable output

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

### JSON output

> Pass flag -j to the program to output in JSON format to stdout. All other messages will be sent to stderr.
> The example below will produce a valid "output.json" file that can be used with other tools.

```bash
GRAFANA_TOKEN=glsa_aaaaaaaaaaa ./detect-angular-dashboards http://my-grafana.example.com/api | tee output.json
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
    "Title": "Angular"
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
    "Title": "Datasource tests - Elasticsearch v7"
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
    "Title": "New dashboard"
  }
]
```

### Using pre-built binaries

You can download pre-built binaries from the [releases](https://github.com/grafana/detect-angular-dashboards/releases) section.

Then, run the program:

```bash
GRAFANA_TOKEN=abcd ./detect-angular-dashboards [GRAFANA_URL=http://127.0.0.1:3000/api]
````

### Building from source

You need to install [Go](https://go.dev) and [Mage](https://magefile.org/).

Then, clone the repository, build and run the program:

```bash
mage build:current
GRAFANA_TOKEN=abcd ./dist/linux_amd64/detect-angular-dashboards [GRAFANA_URL=http://127.0.0.1:3000/api]
```

### Docker image

Clone the repository and build the Docker image:

```bash
docker build -t detect-angular-dashboards .
docker run --rm -it -e GRAFANA_TOKEN=abcd detect-angular-dashboards http://172.17.0.1:3000/api
```


## LICENSE

Apache License 2.0
