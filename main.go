package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/operator-framework/operator-lib/leader"
	"net/http"
	"os"
	"runtime"

	"crypto/tls"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	apis "github.com/redhat-cop/quay-openshift-registry-operator/api"
	"github.com/redhat-cop/quay-openshift-registry-operator/controller"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/constants"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/webhook"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
	// +kubebuilder:scaffold:imports
)

// Change below variables to serve metrics on different host or port.
var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383
)
var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	//log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	//pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	// Add Webhook Flags
	var webhookCrtFile, webhookKeyFile string
	var webhookSslDisable bool

	pflag.StringVar(&webhookCrtFile, "webhookCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	pflag.StringVar(&webhookKeyFile, "webhookKeyFile", "/etc/webhook/certs/key.pem", "File containing the x509 Private Key for HTTPS.")
	pflag.BoolVar(&webhookSslDisable, "webhookSslDisable", false, "Disable Exposing Wehook via SSL (Developer use only).")

	pflag.Parse()

	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	//
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	printVersion()

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	ctx := context.TODO()

	// Become the leader before proceeding
	err = leader.Become(ctx, "quay-openshift-registry-operator-lock")
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		// Namespace:          namespace,
		// MapperProvider:     restmapper.NewDynamicRESTMapper,
		//MapperProvider:     apiutil.NewDynamicRESTMapper,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	if err := imagev1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	if err := buildv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup all Controllers if not running in Webhook only mode

	_, webhookEnvVarFound := os.LookupEnv(constants.WebHookOnlyModeEnabledEnvVar)

	if !webhookEnvVarFound {

		if err := controller.AddToManager(mgr); err != nil {
			log.Error(err, "")
			os.Exit(1)
		}

	}

	// Create Service object to expose the metrics port.
	//_, err = metrics.ExposeMetricsPort(ctx, metricsPort)
	//if err != nil {
	//	log.Info(err.Error())
	//}

	// Enable Webhook support
	_, disableWebhookEnv := os.LookupEnv(constants.DisableWebhookEnvVar)

	if !disableWebhookEnv {

		log.Info("Starting Webhook server")

		codecs := serializer.NewCodecFactory(mgr.GetScheme())

		var whsvr *webhook.WebhookServer

		if !webhookSslDisable {

			pair, err := tls.LoadX509KeyPair(webhookCrtFile, webhookKeyFile)
			if err != nil {
				log.Error(err, "Failed to load key pair")
				os.Exit(1)
			}

			whsvr = &webhook.WebhookServer{
				Server: &http.Server{
					Addr:      ":8443",
					TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
				},
				Deserializer: codecs.UniversalDeserializer(),
				Client:       mgr.GetClient(),
			}
		} else {
			whsvr = &webhook.WebhookServer{
				Server: &http.Server{
					Addr: ":8080",
				},
				Deserializer: codecs.UniversalDeserializer(),
				Client:       mgr.GetClient(),
			}
		}

		// define http server and server handler
		mux := http.NewServeMux()
		mux.HandleFunc("/admissionwebhook", whsvr.Handle)
		whsvr.Server.Handler = mux

		// start webhook server in new routine
		go func() {
			if webhookSslDisable {
				if err := whsvr.Server.ListenAndServe(); err != nil {
					log.Error(err, "Failed to listen and serve webhook server")
				}
			} else {
				if err := whsvr.Server.ListenAndServeTLS("", ""); err != nil {
					log.Error(err, "Failed to listen and serve webhook server")
				}
			}
		}()
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}

}
