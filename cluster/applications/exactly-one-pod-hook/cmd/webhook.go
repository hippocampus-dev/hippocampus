package cmd

import (
	"exactly-one-pod-hook/pkg/webhook"
	"flag"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func webhookCmd() *cobra.Command {
	webhookArgs := webhook.DefaultArgs()

	cmd := &cobra.Command{
		Use:          "webhook",
		Short:        "Start the webhook server",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			webhookArgs.Args = Args
			if err := webhook.Run(webhookArgs); err != nil {
				log.Fatalf("Failed to run webhook.Run: %+v", err)
			}
		},
	}

	cmd.Flags().StringVar(
		&webhookArgs.Host,
		"host",
		webhookArgs.Host,
		"",
	)

	cmd.Flags().IntVar(
		&webhookArgs.Port,
		"port",
		webhookArgs.Port,
		"",
	)

	cmd.Flags().StringVar(
		&webhookArgs.CertDir,
		"certDir",
		webhookArgs.CertDir,
		"CertDir is the directory that contains the server key and certificate. The server key and certificate.",
	)

	cmd.Flags().StringVar(
		&webhookArgs.MetricsAddr,
		"metrics-bind-address",
		webhookArgs.MetricsAddr,
		"The address the metric endpoint binds to.",
	)

	cmd.Flags().BoolVar(
		&webhookArgs.SecureMetrics,
		"metrics-secure",
		webhookArgs.SecureMetrics,
		"If set the metrics endpoint is served securely",
	)

	cmd.Flags().BoolVar(
		&webhookArgs.EnableHTTP2,
		"enable-http2",
		webhookArgs.EnableHTTP2,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers",
	)

	cmd.Flags().StringVar(
		&webhookArgs.ProbeAddr,
		"health-probe-bind-address",
		webhookArgs.ProbeAddr,
		"The address the probe endpoint binds to.",
	)

	cmd.Flags().StringVar(
		&webhookArgs.SidecarImage,
		"sidecar-image",
		webhookArgs.SidecarImage,
		"The image to use for the sidecar container",
	)

	cmd.Flags().BoolVar(
		&webhookArgs.EnableSidecarContainers,
		"enable-sidecar-containers",
		webhookArgs.EnableSidecarContainers,
		"If set, sidecar containers will be enabled",
	)

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	klog.InitFlags(flag.CommandLine)
	flag.Parse()

	zapLogger := zap.New(zap.UseFlagOptions(&opts))
	klog.SetLogger(zapLogger)
	ctrl.SetLogger(zapLogger)

	return cmd
}
