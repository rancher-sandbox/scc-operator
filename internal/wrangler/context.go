package wrangler

import (
	"context"
	"fmt"
	"github.com/rancher-sandbox/scc-operator/internal/settings"
	lasso "github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/lasso/pkg/mapper"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/leader"
	"github.com/rancher/wrangler/v3/pkg/schemes"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sync"

	"github.com/rancher-sandbox/scc-operator/pkg/generated/controllers/management.cattle.io"
	mgmtv3 "github.com/rancher-sandbox/scc-operator/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher-sandbox/scc-operator/pkg/generated/controllers/scc.cattle.io"
	sccv1 "github.com/rancher-sandbox/scc-operator/pkg/generated/controllers/scc.cattle.io/v1"
	managementv3api "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

var (
	localSchemeBuilder = runtime.SchemeBuilder{
		managementv3api.AddToScheme,
		scheme.AddToScheme,
	}
	AddToScheme = localSchemeBuilder.AddToScheme
	Scheme      = runtime.NewScheme()
)

func init() {
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(AddToScheme(Scheme))
	utilruntime.Must(schemes.AddToScheme(Scheme))
}

type MiniContext struct {
	RESTConfig *rest.Config

	Dynamic           *dynamic.DynamicClient
	ControllerFactory controller.SharedControllerFactory
	SharedFactory     lasso.SharedClientFactory
	K8sClient         *kubernetes.Clientset
	Mapper            meta.RESTMapper
	ClientSet         *clientset.Clientset

	Settings *settings.SettingRepo
	Mgmt     mgmtv3.Interface
	Core     corev1.Interface
	SCC      sccv1.Interface

	leadership     *leader.Manager
	controllerLock *sync.Mutex
}

func enableProtobuf(cfg *rest.Config) *rest.Config {
	cpy := rest.CopyConfig(cfg)
	cpy.AcceptContentTypes = "application/vnd.kubernetes.protobuf, application/json"
	cpy.ContentType = "application/json"
	return cpy
}

func NewWranglerMiniContext(ctx context.Context, restConfig *rest.Config) (MiniContext, error) {
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfig(enableProtobuf(restConfig), Scheme)
	if err != nil {
		return MiniContext{}, err
	}

	opts := &generic.FactoryOptions{
		SharedControllerFactory: controllerFactory,
	}

	restmapper, err := mapper.New(restConfig)
	if err != nil {
		return MiniContext{}, fmt.Errorf("error building rest mapper: %s", err.Error())
	}

	clientSet, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return MiniContext{}, fmt.Errorf("error getting clientSet: %s", err.Error())
	}

	k8sclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return MiniContext{}, fmt.Errorf("error getting kubernetes client: %s", err.Error())
	}

	dynamicInterface, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return MiniContext{}, fmt.Errorf("error generating dynamic client: %s", err.Error())
	}
	sharedClientFactory, err := lasso.NewSharedClientFactoryForConfig(restConfig)
	if err != nil {
		return MiniContext{}, fmt.Errorf("error generating shared client factory: %s", err.Error())
	}

	coreF, err := v1core.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return MiniContext{}, fmt.Errorf("error building core sample controllers: %s", err.Error())
	}

	sccFactory, err := scc.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return MiniContext{}, err
	}

	mgmtFactory, err := management.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return MiniContext{}, err
	}

	mgmtInterface := mgmtFactory.Management().V3()

	settingRepo := settings.NewSettingRepository(mgmtInterface.Setting(), mgmtInterface.Setting().Cache())

	k8s, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return MiniContext{}, err
	}

	leadership := leader.NewManager("", "scc-controllers", k8s)

	return MiniContext{
		RESTConfig: restConfig,

		Dynamic:           dynamicInterface,
		ControllerFactory: controllerFactory,
		SharedFactory:     sharedClientFactory,
		K8sClient:         k8sclient,
		Mapper:            restmapper,
		ClientSet:         clientSet,

		Settings: settingRepo,
		Mgmt:     mgmtInterface,
		Core:     coreF.Core().V1(),
		SCC:      sccFactory.Scc().V1(),

		leadership:     leadership,
		controllerLock: &sync.Mutex{},
	}, nil
}

func (c *MiniContext) Start(ctx context.Context) error {
	c.controllerLock.Lock()
	defer c.controllerLock.Unlock()

	if err := c.ControllerFactory.Start(ctx, 50); err != nil {
		return err
	}
	c.leadership.Start(ctx)
	return nil
}

func (c *MiniContext) OnLeader(f func(ctx context.Context) error) {
	c.leadership.OnLeader(f)
}
