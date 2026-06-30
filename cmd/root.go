package cmd

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/Diniboy1123/usque/config"
	"github.com/Diniboy1123/usque/internal"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "usque",
	Short: "Usque Warp CLI",
	Long:  "An unofficial Cloudflare Warp CLI that uses the MASQUE protocol and exposes the tunnel as various different services.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		configPath, err := cmd.Flags().GetString("config")
		if err != nil {
			log.Fatalf("Failed to get config path: %v", err)
		}

		if configPath != "" {
			if err := config.LoadConfig(configPath); err != nil {
				log.Printf("Config file not found: %v", err)
				log.Printf("You may only use the register command to generate one.")
			}
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	internal.InstallDefaultLogTZStamp()
	rootCmd.PersistentFlags().StringP("config", "c", "config.json", "config file (default is config.json)")

	// Optional escape hatch for environments where the system's DNS or TUN stack
	// hijacks outbound API traffic (FakeIP, transparent proxies). When set, the
	// default HTTP client used for Cloudflare API calls dials TCP via the named
	// physical interface via SO_BINDTOIF. MASQUEdial sockets are unaffected.
	if iface := os.Getenv("USQUE_BIND_IFACE"); iface != "" {
		dialer, err := internal.BoundDialer(iface)
		if err != nil {
			log.Printf("USQUE_BIND_IFACE=%q: %v; falling back to default dialer", iface, err)
		} else {
			http.DefaultClient = &http.Client{
				Transport: &http.Transport{
					Proxy:                 http.ProxyFromEnvironment,
					DialContext:           dialer.DialContext,
					ForceAttemptHTTP2:     true,
					TLSHandshakeTimeout:   0,
					DisableCompression:    true,
					MaxIdleConns:          10,
					IdleConnTimeout:       30,
					ResponseHeaderTimeout: 0,
					ExpectContinueTimeout: 0,
				},
			}
			log.Printf("HTTP client bound to interface %s", iface)
		}
	}
}

// Keep the net import available for future diagnostic use.
var _ = net.IPv4len
