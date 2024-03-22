# Releasing a new version
Releases are done through [Drone](https://drone.grafana.net/grafana/detect-angular-dashboards).

Once the changes are into the `main` branch and you're ready to cut a new release, create and push an annotated tag for the new release:

```bash
git tag -a v0.7.0 -m "v0.7.0"
git push origin v0.7.0
```

At this point, a Drone pipeline will start and create a GitHub Release. You can follow its progress [here](https://drone.grafana.net/grafana/detect-angular-dashboards).

Once the release appears in the GitHub "[releases](https://github.com/grafana/detect-angular-dashboards/releases)" section, you can manually edit its description to list the relevant changes.
