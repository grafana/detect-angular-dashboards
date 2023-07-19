package gcom

type PluginVersion struct {
	Version         string
	AngularDetected bool
}

type PluginVersions struct {
	Items []PluginVersion
}
