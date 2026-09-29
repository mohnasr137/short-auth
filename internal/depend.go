package internal

import (
	"log"
	"net/http"

	"mohnasr137/short-auth/config"

	"github.com/coreos/go-oidc/v3/oidc"
)

type Application struct {
	InfoLog    *log.Logger
	ErrorLog   *log.Logger
	Verifier   *oidc.IDTokenVerifier
	Config     *config.Config
	HTTPClient *http.Client
}
