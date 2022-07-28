package authz

import (
	"errors"
	"layer4balancer/config"
	"strings"

	log "github.com/sirupsen/logrus"
)

// AuthzRule defines an authorization rule.
type AuthzRule struct {
	IsAllowed    bool
	CommonName   string
	UpstreamAddr string // ip:port
}

type AuthzScheme struct {
	Rules []AuthzRule
}

func New(cfg config.AuthzCfg) (AuthzScheme, error) {

	authzScheme := AuthzScheme{
		Rules: make([]AuthzRule, 0),
	}

	for _, rule := range cfg.Rules {
		// parse rules
		parts := strings.Split(rule, "-")
		if len(parts) != 3 {
			log.Error("Bad authz rule format", rule)
			return AuthzScheme{}, errors.New("Bad authz rule format: " + rule)
		}

		commonName := strings.TrimSpace(parts[0])
		isAllowed := strings.TrimSpace(parts[1])
		upstreamAddr := strings.TrimSpace(parts[2])

		if isAllowed != "allow" && isAllowed != "deny" {
			log.Error("Unsupported isAllow value: ", isAllowed)
			return AuthzScheme{}, errors.New("Unsupported isAllow value: " + isAllowed)
		}

		rule := AuthzRule{
			IsAllowed:    isAllowed == "allow",
			CommonName:   commonName,
			UpstreamAddr: upstreamAddr,
		}

		authzScheme.Rules = append(authzScheme.Rules, rule)
	}
	return authzScheme, nil
}

func (a *AuthzScheme) Allows(commonName string, upstreamAddr string) bool {
	for _, r := range a.Rules {
		if strings.Compare(r.CommonName, commonName) == 0 && strings.Compare(r.UpstreamAddr, upstreamAddr) == 0 {
			return r.IsAllowed
		}
	}
	// if no matches found, by default, allow access
	return true
}
