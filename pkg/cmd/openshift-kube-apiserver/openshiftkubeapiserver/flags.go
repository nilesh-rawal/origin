package openshiftkubeapiserver

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	"github.com/openshift/origin/pkg/cmd/configflags"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/configconversion"
	"k8s.io/apimachinery/pkg/runtime"
)

func ConfigToFlags(kubeAPIServerConfig *kubecontrolplanev1.KubeAPIServerConfig) ([]string, error) {
	args := unmaskArgs(kubeAPIServerConfig.APIServerArguments)

	host, portString, err := net.SplitHostPort(kubeAPIServerConfig.ServingInfo.BindAddress)
	if err != nil {
		return nil, err
	}

	// TODO this list (and the content below) will be used to drive a config struct and a reflective test matching config to flags
	// these flags are overridden by a patch
	// admission-control
	// authentication-token-webhook-cache-ttl
	// authentication-token-webhook-config-file
	// authorization-mode
	// authorization-policy-file
	// authorization-webhook-cache-authorized-ttl
	// authorization-webhook-cache-unauthorized-ttl
	// authorization-webhook-config-file
	// basic-auth-file
	// enable-aggregator-routing
	// enable-bootstrap-token-auth
	// oidc-client-id
	// oidc-groups-claim
	// oidc-groups-prefix
	// oidc-issuer-url
	// oidc-required-claim
	// oidc-signing-algs
	// oidc-username-claim
	// oidc-username-prefix
	// service-account-lookup
	// token-auth-file

	// alsologtostderr - don't know whether to change it
	// apiserver-count - ignored, hopefully we don't have to fix via patch
	// cert-dir - ignored because we set certs

	// these flags were never supported via config
	// cloud-config
	// cloud-provider
	// cloud-provider-gce-lb-src-cidrs
	// contention-profiling
	// default-not-ready-toleration-seconds
	// default-unreachable-toleration-seconds
	// default-watch-cache-size
	// delete-collection-workers
	// deserialization-cache-size
	// enable-garbage-collector
	// etcd-compaction-interval
	// etcd-count-metric-poll-period
	// etcd-servers-overrides
	// experimental-encryption-provider-config
	// feature-gates
	// http2-max-streams-per-connection
	// insecure-bind-address
	// kubelet-timeout
	// log-backtrace-at
	// log-dir
	// log-flush-frequency
	// logtostderr
	// master-service-namespace
	// max-connection-bytes-per-sec
	// profiling
	// request-timeout
	// runtime-config
	// service-account-api-audiences
	// service-account-issuer
	// service-account-key-file
	// service-account-max-token-expiration
	// service-account-signing-key-file
	// stderrthreshold
	// storage-versions
	// target-ram-mb
	// v
	// version
	// vmodule
	// watch-cache
	// watch-cache-sizes

	// TODO, we need to set these in order to enable the right admission plugins in each of the servers
	// TODO this is needed for a viable cluster up
	admissionFlags, err := admissionFlags(kubeAPIServerConfig.AdmissionPluginConfig)
	if err != nil {
		return nil, err
	}
	for flag, value := range admissionFlags {
		configflags.SetIfUnset(args, flag, value...)
	}
	configflags.SetIfUnset(args, "allow-privileged", "true")
	configflags.SetIfUnset(args, "anonymous-auth", "false")
	configflags.SetIfUnset(args, "authorization-mode", "RBAC", "Node") // overridden later, but this runs the poststarthook for bootstrapping RBAC
	for flag, value := range configflags.AuditFlags(&kubeAPIServerConfig.AuditConfig, configflags.ArgsWithPrefix(args, "audit-")) {
		configflags.SetIfUnset(args, flag, value...)
	}
	configflags.SetIfUnset(args, "bind-address", host)
	configflags.SetIfUnset(args, "client-ca-file", kubeAPIServerConfig.ServingInfo.ClientCA)
	configflags.SetIfUnset(args, "cors-allowed-origins", kubeAPIServerConfig.CORSAllowedOrigins...)
	configflags.SetIfUnset(args, "enable-logs-handler", "false")
	configflags.SetIfUnset(args, "enable-swagger-ui", "true")
	configflags.SetIfUnset(args, "endpoint-reconciler-type", "lease")
	configflags.SetIfUnset(args, "etcd-cafile", kubeAPIServerConfig.StorageConfig.CA)
	configflags.SetIfUnset(args, "etcd-certfile", kubeAPIServerConfig.StorageConfig.CertFile)
	configflags.SetIfUnset(args, "etcd-keyfile", kubeAPIServerConfig.StorageConfig.KeyFile)
	configflags.SetIfUnset(args, "etcd-prefix", kubeAPIServerConfig.StorageConfig.StoragePrefix)
	configflags.SetIfUnset(args, "etcd-servers", kubeAPIServerConfig.StorageConfig.URLs...)
	configflags.SetIfUnset(args, "event-ttl", "3h") // set a TTL long enough to last for our CI tests so we see the first set of events.
	configflags.SetIfUnset(args, "insecure-port", "0")
	configflags.SetIfUnset(args, "kubelet-certificate-authority", kubeAPIServerConfig.KubeletClientInfo.CA)
	configflags.SetIfUnset(args, "kubelet-client-certificate", kubeAPIServerConfig.KubeletClientInfo.CertFile)
	configflags.SetIfUnset(args, "kubelet-client-key", kubeAPIServerConfig.KubeletClientInfo.KeyFile)
	configflags.SetIfUnset(args, "kubelet-https", "true")
	configflags.SetIfUnset(args, "kubelet-preferred-address-types", "Hostname", "InternalIP", "ExternalIP")
	configflags.SetIfUnset(args, "kubelet-read-only-port", "0")
	configflags.SetIfUnset(args, "kubernetes-service-node-port", "0")
	configflags.SetIfUnset(args, "max-mutating-requests-inflight", fmt.Sprintf("%d", kubeAPIServerConfig.ServingInfo.MaxRequestsInFlight/2))
	configflags.SetIfUnset(args, "max-requests-inflight", fmt.Sprintf("%d", kubeAPIServerConfig.ServingInfo.MaxRequestsInFlight))
	configflags.SetIfUnset(args, "min-request-timeout", fmt.Sprintf("%d", kubeAPIServerConfig.ServingInfo.RequestTimeoutSeconds))
	configflags.SetIfUnset(args, "proxy-client-cert-file", kubeAPIServerConfig.AggregatorConfig.ProxyClientInfo.CertFile)
	configflags.SetIfUnset(args, "proxy-client-key-file", kubeAPIServerConfig.AggregatorConfig.ProxyClientInfo.KeyFile)
	configflags.SetIfUnset(args, "requestheader-allowed-names", kubeAPIServerConfig.AuthConfig.RequestHeader.ClientCommonNames...)
	configflags.SetIfUnset(args, "requestheader-client-ca-file", kubeAPIServerConfig.AuthConfig.RequestHeader.ClientCA)
	configflags.SetIfUnset(args, "requestheader-extra-headers-prefix", kubeAPIServerConfig.AuthConfig.RequestHeader.ExtraHeaderPrefixes...)
	configflags.SetIfUnset(args, "requestheader-group-headers", kubeAPIServerConfig.AuthConfig.RequestHeader.GroupHeaders...)
	configflags.SetIfUnset(args, "requestheader-username-headers", kubeAPIServerConfig.AuthConfig.RequestHeader.UsernameHeaders...)
	configflags.SetIfUnset(args, "secure-port", portString)
	configflags.SetIfUnset(args, "service-cluster-ip-range", kubeAPIServerConfig.ServicesSubnet)
	configflags.SetIfUnset(args, "service-node-port-range", kubeAPIServerConfig.ServicesNodePortRange)
	configflags.SetIfUnset(args, "storage-backend", "etcd3")
	configflags.SetIfUnset(args, "storage-media-type", "application/vnd.kubernetes.protobuf")
	configflags.SetIfUnset(args, "tls-cert-file", kubeAPIServerConfig.ServingInfo.CertFile)
	configflags.SetIfUnset(args, "tls-cipher-suites", kubeAPIServerConfig.ServingInfo.CipherSuites...)
	configflags.SetIfUnset(args, "tls-min-version", kubeAPIServerConfig.ServingInfo.MinTLSVersion)
	configflags.SetIfUnset(args, "tls-private-key-file", kubeAPIServerConfig.ServingInfo.KeyFile)
	configflags.SetIfUnset(args, "tls-sni-cert-key", sniCertKeys(kubeAPIServerConfig.ServingInfo.NamedCertificates)...)
	configflags.SetIfUnset(args, "secure-port", portString)

	return configflags.ToFlagSlice(args), nil
}

func admissionFlags(admissionPluginConfig map[string]configv1.AdmissionPluginConfig) (map[string][]string, error) {
	args := map[string][]string{}

	forceOn := []string{}
	forceOff := []string{}
	pluginConfig := map[string]configv1.AdmissionPluginConfig{}
	for pluginName, origConfig := range admissionPluginConfig {
		config := *origConfig.DeepCopy()
		if len(config.Location) > 0 {
			content, err := ioutil.ReadFile(config.Location)
			if err != nil {
				return nil, err
			}
			// if the config isn't a DefaultAdmissionConfig, then assume we're enabled (we were called out after all)
			// if the config *is* a DefaultAdmissionConfig and it explicitly said to disable us, we are disabled
			obj, err := configapilatest.ReadYAML(bytes.NewBuffer(content))
			// if we can't read it, let the plugin deal with it
			// if nothing was there, let the plugin deal with it
			if err != nil || obj == nil {
				forceOn = append(forceOn, pluginName)
				config.Location = ""
				config.Configuration = runtime.RawExtension{Raw: content}
				pluginConfig[pluginName] = config
				continue
			}

			if defaultConfig, ok := obj.(*configapi.DefaultAdmissionConfig); !ok {
				forceOn = append(forceOn, pluginName)
				config.Location = ""
				config.Configuration = runtime.RawExtension{Raw: content}
				pluginConfig[pluginName] = config

			} else if defaultConfig.Disable {
				forceOff = append(forceOff, pluginName)

			} else {
				forceOn = append(forceOn, pluginName)
			}

			continue
		}

		// if it wasn't a DefaultAdmissionConfig object, let the plugin deal with it
		currConfig := &configapi.DefaultAdmissionConfig{}
		uncastDefaultConfig, _, decodingErr := configapilatest.Codec.Decode(config.Configuration.Raw, nil, currConfig)
		if decodingErr != nil {
			forceOn = append(forceOn, pluginName)
			pluginConfig[pluginName] = config
			continue
		}

		defaultConfig, ok := uncastDefaultConfig.(*configapi.DefaultAdmissionConfig)
		if !ok {
			forceOn = append(forceOn, pluginName)
			pluginConfig[pluginName] = config

		} else if defaultConfig.Disable {
			forceOff = append(forceOff, pluginName)

		} else {
			forceOn = append(forceOn, pluginName)
		}

	}
	upstreamAdmissionConfig, err := configconversion.ConvertOpenshiftAdmissionConfigToKubeAdmissionConfig(pluginConfig)
	if err != nil {
		return nil, err
	}
	configBytes, err := configapilatest.WriteYAML(upstreamAdmissionConfig)
	if err != nil {
		return nil, err
	}

	tempFile, err := ioutil.TempFile("", "kubeapiserver-admission-config.yaml")
	if err != nil {
		return nil, err
	}
	if _, err := tempFile.Write(configBytes); err != nil {
		return nil, err
	}
	tempFile.Close()

	configflags.SetIfUnset(args, "admission-control-config-file", tempFile.Name())
	configflags.SetIfUnset(args, "disable-admission-plugins", forceOff...)
	configflags.SetIfUnset(args, "enable-admission-plugins", forceOn...)

	return args, nil
}

func sniCertKeys(namedCertificates []configv1.NamedCertificate) []string {
	args := []string{}
	for _, nc := range namedCertificates {
		names := ""
		if len(nc.Names) > 0 {
			names = ":" + strings.Join(nc.Names, ",")
		}
		args = append(args, fmt.Sprintf("%s,%s%s", nc.CertFile, nc.KeyFile, names))
	}
	return args
}

func unmaskArgs(args map[string]kubecontrolplanev1.Arguments) map[string][]string {
	ret := map[string][]string{}
	for key, slice := range args {
		for _, val := range slice {
			ret[key] = append(ret[key], val)
		}
	}
	return ret
}
