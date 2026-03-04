package protocol

// VersionCheckParams は version.check リクエストのパラメータ。
type VersionCheckParams struct{}

// VersionCheckResult は version.check リクエストの結果。
type VersionCheckResult struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url,omitempty"`
	CheckedAt       string `json:"checked_at,omitempty"`
}
