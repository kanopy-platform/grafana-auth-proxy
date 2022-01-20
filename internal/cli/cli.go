package cli

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/jwt"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/server"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/config"
	"github.com/kanopy-platform/grafana-auth-proxy/pkg/grafana"
)

type RootCommand struct{}

func NewRootCommand() *cobra.Command {
	root := &RootCommand{}

	cmd := &cobra.Command{
		Use:               "grafana-auth-proxy",
		PersistentPreRunE: root.persistentPreRunE,
		RunE:              root.runE,
	}

	cmd.PersistentFlags().String("log-level", "info", "Configure log level")
	cmd.PersistentFlags().String("listen-address", ":8080", "Server listen address")
	cmd.PersistentFlags().Duration("http-client-timeout", 60*time.Second, "HTTP Client timeout in seconds")
	cmd.PersistentFlags().Bool("tls-skip-verify", false, "Skip TLS certificate verification")
	cmd.PersistentFlags().String("grafana-proxy-url", "http://grafana.example.com", "Grafana url to proxy to")
	cmd.PersistentFlags().String("grafana-user-header", "X-WEBAUTH-USER", "Header to containing the user to authenticate")
	cmd.PersistentFlags().String("cookie-name", "auth_token", "Name of http Cookie containing the jwt token")
	cmd.PersistentFlags().String("header-name", "Authorization", "Name of http header containing the jwt token")
	cmd.PersistentFlags().StringSlice("jwt-containers", []string{"cookie", "header"}, "Slice of jwt containers to try, in order")
	cmd.PersistentFlags().String("admin-user", "admin", "Admin user")
	cmd.PersistentFlags().String("admin-password", "", "Admin password")
	cmd.PersistentFlags().String("jwt-claim-login", "email", "JWT claim to be used as user Login in Grafana. Valid values are 'email' or 'sub'")
	cmd.PersistentFlags().String("jwt-claim-name", "sub", "JWT claim to be used as user Name in Grafana. Valid values are 'email' or 'sub'")

	return cmd
}

func (c *RootCommand) persistentPreRunE(cmd *cobra.Command, args []string) error {
	// Read from config
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/grafana-auth-proxy/")
	// viper.WatchConfig()

	err := viper.ReadInConfig()
	if err != nil {
		log.Error("error reading config file, ", err)
		return err
	}
	log.Info("Using config file ", viper.GetViper().ConfigFileUsed())

	// bind flags to viper
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.SetEnvPrefix("app")
	viper.AutomaticEnv()

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	// set log level
	logLevel, err := log.ParseLevel(viper.GetString("log-level"))
	if err != nil {
		return err
	}

	log.SetLevel(logLevel)

	return nil
}

func defaultServerOptions() []server.ServerFuncOpt {
	responseHeaders := server.GrafanaResponseHeaders{
		User: viper.GetString("grafana-user-header"),
	}

	jwtContainers := viper.GetStringSlice("jwt-containers")

	// HACK: Viper doesn't set the slice correctly when using an env variable
	// so it ends up with a string containing commas at index 0
	if strings.Contains(jwtContainers[0], ",") {
		jwtContainers = strings.Split(jwtContainers[0], ",")
	}

	containers := buildJWTContainers(jwtContainers)

	opts := []server.ServerFuncOpt{
		server.WithGrafanaResponseHeaders(responseHeaders),
		server.WithTokenContainers(containers),
	}

	return opts
}

func isValidClaimKey(key string) error {
	if key == "sub" || key == "email" {
		return nil
	} else {
		return fmt.Errorf("%s can only have a value of \"sub\" or \"email\"", key)
	}
}

func buildJWTContainers(jwtContainers []string) []jwt.TokenContainer {
	var output []jwt.TokenContainer

	for _, ctype := range jwtContainers {
		switch ctype {
		case "cookie":
			output = append(output, jwt.NewCookieContainer(viper.GetString("cookie-name")))
		case "header":
			output = append(output, jwt.NewHeaderContainer(viper.GetString("header-name")))
		}
	}

	return output
}

func (c *RootCommand) runE(cmd *cobra.Command, args []string) error {
	addr := viper.GetString("listen-address")
	log.Infof("listening on %s", addr)

	opts := defaultServerOptions()

	grafanaProxyURL, err := url.Parse(viper.GetString("grafana-proxy-url"))
	if err != nil {
		log.Error("grafana-proxy-url is not a proper url")
		return err
	}
	opts = append(opts, server.WithGrafanaProxyURL(grafanaProxyURL))

	adminPassword := viper.GetString("admin-password")
	if adminPassword == "" {
		log.Error("admin-password is not set")
		return err
	}

	loginKey := viper.GetString("jwt-claim-login")
	err = isValidClaimKey(loginKey)
	if err != nil {
		log.WithError(err)
		return err
	}

	nameKey := viper.GetString("jwt-claim-name")
	err = isValidClaimKey(nameKey)
	if err != nil {
		log.WithError(err)
		return err
	}

	claimsMap := server.GrafanaClaimsConfig{
		Login: loginKey,
		Name:  nameKey,
	}
	opts = append(opts, server.WithGrafanaClaimsConfig(claimsMap))

	grafanaHTTPClient := http.DefaultClient
	grafanaHTTPClient.Timeout = viper.GetDuration("http-client-timeout")

	grafanaConfig := gapi.Config{
		BasicAuth: url.UserPassword(viper.GetString("admin-user"), adminPassword),
		Client:    grafanaHTTPClient,
	}

	grafanaClient, err := grafana.NewClient(grafanaProxyURL, grafanaConfig)
	if err != nil {
		log.Error("error creating Grafana client, ", err)
		return err
	}
	opts = append(opts, server.WithGrafanaClient(grafanaClient))

	groups := config.Groups{}
	if err := viper.UnmarshalKey("groups", &groups); err != nil {
		log.Error("error parsing groups settings in config, ", err)
		return err
	}

	opts = append(opts, server.WithConfigGroups(groups))
	log.Debugf("groups configuration map: %v", groups)

	skipTLSVerify := viper.GetBool("tls-skip-verify")
	if skipTLSVerify {
		opts = append(opts, server.SkipTLSVerify())
	}

	s, err := server.New(opts...)
	if err != nil {
		return err
	}

	return http.ListenAndServe(addr, s)
}
