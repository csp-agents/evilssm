package network

import (
	"fmt"
	"net/http"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

func GetDefaultTransport(log log.T, appConfig appconfig.SsmagentConfig) *http.Transport {
	result := http.DefaultTransport.(*http.Transport).Clone()
	result.TLSClientConfig = GetDefaultTLSConfig(log, appConfig)
	return result
}

func DisableHTTPDowngrade(req *http.Request, via []*http.Request) error {
	//Go's http.DefaultClient allows 10 redirects before returning an error.
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}

	//Send an error on HTTP redirect attempt
	if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
		lastHop := via[len(via)-1].URL
		return fmt.Errorf("redirected from secure URL %s to insecure URL %s", lastHop, req.URL)
	}
	return nil
}
