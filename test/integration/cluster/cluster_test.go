package deployment

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	informers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clientset "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	"testing"
)

func TestCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cluster-Controller")
}

var clusterInstance = &clusterv1alpha1.Cluster{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "foo",
		Namespace: "clusterapi-test",
	},
	Spec: clusterv1alpha1.ClusterSpec{
		ClusterNetwork: clusterv1alpha1.ClusterNetworkingConfig{
			ServiceDomain: "mydomain.com",
			Services: clusterv1alpha1.NetworkRanges{
				CIDRBlocks: []string{"10.96.0.0/12"},
			},
			Pods: clusterv1alpha1.NetworkRanges{
				CIDRBlocks: []string{"192.168.0.0/16"},
			},
		},
	},
}

var testNamespace = "clusterapi-test"

var _ = Describe("Cluster-Controller", func() {
	var clusterapi client.ClusterInterface
	var client *kubernetes.Clientset
	var stopper chan struct{}
	var informer cache.SharedIndexInformer

	BeforeEach(func() {

		// Load configuration
		kubeconfig := os.Getenv("KUBECONFIG")
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		Expect(err).ShouldNot(HaveOccurred())

		// Create kubernetes client
		client, err = kubernetes.NewForConfig(config)
		Expect(err).ShouldNot(HaveOccurred())

		// Create namespace for test
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
		_, err = client.Core().Namespaces().Create(ns)
		Expect(err).ShouldNot(HaveOccurred())

		// Create  informer for events in the namespace
		factory := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithNamespace(testNamespace))
		informer = factory.Core().V1().Events().Informer()
		stopper = make(chan struct{})

		// Create clusterapi client
		cs, err := clientset.NewForConfig(config)
		Expect(err).ShouldNot(HaveOccurred())
		clusterapi = cs.ClusterV1alpha1().Clusters(testNamespace)
	})

	AfterEach(func() {
		close(stopper)
		client.Core().Namespaces().Delete(testNamespace, &metav1.DeleteOptions{})
	})

	Describe("Create Cluster", func() {
		It("Should trigger an event", func() {
			// Register handler for cluster events
			events := make(chan *corev1.Event, 0)
			informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					e := obj.(*corev1.Event)
					if e.InvolvedObject.Kind == "Cluster" {
						events <- e
					}
				},
			})
			go informer.Run(stopper)

			// Create Cluster
			cluster := clusterInstance.DeepCopy()
			_, err := clusterapi.Create(cluster)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(<-events).NotTo(BeNil())

		})
	})
})
