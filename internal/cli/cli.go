package cli

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/config"
	"github.com/kanopy-platform/grafana-auth-proxy/internal/server"
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
	cmd.PersistentFlags().String("cookie-name", "auth_token", "Cookie name with jwt token. If set will take precedence over auth header")
	cmd.PersistentFlags().String("admin-user", "admin", "Admin user")
	cmd.PersistentFlags().String("admin-password", "", "Admin password")

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

	opts := []server.ServerFuncOpt{
		server.WithCookieName(viper.GetString("cookie-name")),
		server.WithGrafanaResponseHeaders(responseHeaders),
	}

	return opts
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
