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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/self-host/self-host/api/aapije/rest"
)

func printUsers(users []rest.User, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(users)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "UUID\tNAME\tGROUPS")
		for _, user := range users {
			fmt.Fprintf(w, "%s\t%s\t%s\n", user.Uuid, user.Name, joinGroupNames(user.Groups))
		}
		return w.Flush()
	}
}

func printUser(user *rest.User, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(user)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "UUID:\t%s\n", user.Uuid)
		fmt.Fprintf(w, "Name:\t%s\n", user.Name)
		fmt.Fprintf(w, "Groups:\t%s\n", joinGroupNames(user.Groups))
		return w.Flush()
	}
}

func printTokens(tokens []rest.Token, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(tokens)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "UUID\tNAME\tCREATED")
		for _, token := range tokens {
			fmt.Fprintf(w, "%s\t%s\t%s\n", token.Uuid, token.Name, token.Created.Format("2006-01-02 15:04:05"))
		}
		return w.Flush()
	}
}

func printTokenWithSecret(token *rest.TokenWithSecret, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(token)
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "UUID:\t%s\n", token.Uuid)
		fmt.Fprintf(w, "Name:\t%s\n", token.Name)
		fmt.Fprintf(w, "Secret:\t%s\n", token.Secret)
		return w.Flush()
	}
}

func joinGroupNames(groups []rest.Group) string {
	if len(groups) == 0 {
		return ""
	}

	names := make([]string, 0, len(groups))
	for _, group := range groups {
		names = append(names, group.Name)
	}
	return strings.Join(names, ", ")
}
