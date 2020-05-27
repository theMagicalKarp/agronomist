package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/theMagicalKarp/agronomist/pkg/reconciler"
	"github.com/theMagicalKarp/agronomist/pkg/storage"
)


func RootCMD() *cobra.Command {
	command := &cobra.Command{
		Use:   "agronomist",
		Short: "agronomist autoscales Kubernetes pods using OPA",
		Run: Run,
	}

	flags := command.Flags()
	flags.StringP(
		"kubeconfig", "", "", "Path to kubeconfig/assumes in-cluster if not provided",
	)

	flags.StringP(
		"pod", "", "local", "Name of pod which agronomist is running in",
	)
	flags.StringP(
		"pod-uid", "", "11111111-1111-1111-1111-111111111111", "UID of pod which agronomist is running in",
	)
	flags.StringP(
		"namespace", "", "kube-system", "Namespace of which agronomist is running in",
	)

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	viper.BindPFlags(flags)

	return command
}


func Run(cmd *cobra.Command, args []string) {
	rego.RegisterBuiltin1(
		&rego.Function{
			Name: "parseunit",
			Decl: types.NewFunction(types.Args(types.S), types.N),
		},
		func(_ rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {
			if str, ok := a.Value.(ast.String); ok {
				quantity, err := resource.ParseQuantity(string(str))

				if err != nil {
					return nil, nil
				}
				return ast.IntNumberTerm(int(quantity.ScaledValue(resource.Milli))), nil
			}
			return nil, nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())


	kubeconfig := viper.GetString("kubeconfig")
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	metricsClientset, err := metricsv.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	dynamicClientset, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	dynamicFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClientset, 0, metav1.NamespaceAll, nil)

	factory := informers.NewSharedInformerFactory(clientset, time.Hour*24)

	store := storage.NewStore(clientset, metricsClientset, dynamicClientset, factory, dynamicFactory)
	store.Start(ctx)

	scalingPolicyReconciler := reconciler.CreateScalingPolicyReconciler(
		viper.GetString("namespace"),
		viper.GetString("pod"),
		k8stypes.UID(viper.GetString("pod-uid")),
		store,
	)

	go scalingPolicyReconciler.Start(ctx)

	fmt.Println("Starting!")

	sigCh := make(chan os.Signal, 0)
	signal.Notify(sigCh, os.Kill, os.Interrupt)
	<-sigCh
	cancel()
}
