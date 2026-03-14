/*
Copyright © 2021 Self-host Authors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/self-host/self-host/api/aapije/rest"
	"github.com/spf13/viper"
)

type apiConnectionOptions struct {
	Server string
	Domain string
	Token  string
}

func resolveAPIConnection(serverFlag, domainFlag, tokenFlag string) (apiConnectionOptions, error) {
	server := strings.TrimSpace(serverFlag)
	if server == "" {
		server = strings.TrimSpace(viper.GetString("api.server"))
	}
	if server == "" {
		return apiConnectionOptions{}, fmt.Errorf("missing api server; set --server or api.server in config")
	}

	domain := strings.TrimSpace(domainFlag)
	if domain == "" {
		domain = strings.TrimSpace(viper.GetString("api.domain"))
	}
	if domain == "" {
		return apiConnectionOptions{}, fmt.Errorf("missing api domain; set --domain or api.domain in config")
	}

	token := strings.TrimSpace(tokenFlag)
	if token == "" {
		token = strings.TrimSpace(viper.GetString("api.token"))
	}
	if token == "" {
		return apiConnectionOptions{}, fmt.Errorf("missing api token; set --token or api.token in config")
	}

	return apiConnectionOptions{
		Server: server,
		Domain: domain,
		Token:  token,
	}, nil
}

func newAPIClient(cfg apiConnectionOptions) (*rest.ClientWithResponses, error) {
	return rest.NewClientWithResponses(cfg.Server, rest.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.SetBasicAuth(cfg.Domain, cfg.Token)
		return nil
	}))
}
