package config

import "strings"

const (
	DefaultPanelGitHubRepository = "https://github.com/zcw199604/Cli-Proxy-API-Management-Center"
	LegacyPanelGitHubRepository  = "https://github.com/router-for-me/Cli-Proxy-API-Management-Center"
	DefaultPprofAddr             = "127.0.0.1:8316"
	DefaultAuthDir               = "~/.cli-proxy-api"
	DefaultUsageStatsDBPath      = "data/usage.db"
	DefaultPricingDataDir        = "data"
	DefaultPricingSyncInterval   = "24h"
)

func normalizePanelGitHubRepository(repository string) string {
	repository = strings.TrimSpace(repository)
	if repository == "" || strings.EqualFold(repository, LegacyPanelGitHubRepository) {
		return DefaultPanelGitHubRepository
	}
	return repository
}
