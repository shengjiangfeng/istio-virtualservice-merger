/*
 * Copyright 2021 - now, the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *       https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"fmt"
	"github.com/go-logr/logr"
	"github.com/monimesl/operator-helper/oputil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strconv"
	"strings"
	"sync"
)

var logger logr.Logger
var loggerOnce sync.Once

var envEnableWebHooks = "ENABLE_WEBHOOKS"
var envWebHookCertificateDir = "WEBHOOK_CERTIFICATES_DIR"
var envNamespacesToWatch = "NAMESPACES_TO_WATCH"
var envEnableLeaderElection = "ENABLE_LEADER_ELECTION"
var envLeaderElectionNamespace = "LEADER_ELECTION_NAMESPACE"
var envMetricsServerPort = "METRICS_SERVER_PORT"

// RequireRootLogger get the root logger or panic if not yet created
func RequireRootLogger() logr.Logger {
	return logger
}

// GetLogger get the logger instance to use
func GetLogger(operatorName string, opts ...zap.Opts) logr.Logger {
	loggerOnce.Do(func() {
		if len(opts) == 0 {
			opts = append(opts, zap.UseDevMode(true))
		}
		logger = zap.New(opts...).WithName(operatorName)
		ctrl.SetLogger(logger)
	})
	return logger
}

// NewRestConfig creates new rest config or panic
func NewRestConfig() *rest.Config {
	return config.GetConfigOrDie()
}

// RequireRestClient creates a singleton rest interface
func RequireRestClient() rest.Interface {
	return RequireClientset().RESTClient()
}

// RequireClientset creates a singleton client set
func RequireClientset() *kubernetes.Clientset {
	cfg := NewRestConfig()
	return kubernetes.NewForConfigOrDie(cfg)
}

// GetManagerParams get the manager options to use
func GetManagerParams(scheme *runtime.Scheme, operatorName, domainName string) (*rest.Config, ctrl.Options) {
	options := ctrl.Options{
		Scheme:                  scheme,
		Port:                    9443,
		MetricsBindAddress:      metricServerAddress(),
		Logger:                  GetLogger(operatorName),
		LeaderElection:          LeaderElectionEnabled(),
		LeaderElectionNamespace: LeaderElectionNamespace(operatorName),
		LeaderElectionID:        fmt.Sprintf("leader-lock-65403bab.%s.%s", operatorName, domainName),
	}
	namespaces := NamespacesToWatch()
	switch {
	case len(namespaces) == 0:
		options.Namespace = ""
	case len(namespaces) == 1:
		options.Namespace = namespaces[0]
	default:
		options.NewCache = cache.MultiNamespacedCacheBuilder(namespaces)
	}
	options.CertDir = GetWebHookCertDir()
	return NewRestConfig(), options
}

// LeaderElectionEnabled checks if leader election is enabled
func LeaderElectionEnabled() bool {
	return strings.TrimSpace(os.Getenv(envEnableLeaderElection)) != "false"
}

func metricServerAddress() string {
	portStr := strings.TrimSpace(os.Getenv(envMetricsServerPort))
	if portStr == "" {
		return ""
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("Invalid %s=%s", envMetricsServerPort, portStr)
	}
	return fmt.Sprintf(":%d", port)
}

// WebHooksEnabled checks if webhook is enabled
func WebHooksEnabled() bool {
	if strings.TrimSpace(os.Getenv(envEnableWebHooks)) != "false" {
		if _, err := os.Stat(GetWebHookCertDir()); !os.IsNotExist(err) {
			return true
		}
		log.Printf("The webhook cert directory does not exists: %s", GetWebHookCertDir())
	}
	return strings.TrimSpace(os.Getenv(envEnableWebHooks)) != "false"
}

// LeaderElectionNamespace get the leader election namespace
func LeaderElectionNamespace(operatorName string) string {
	ns := strings.TrimSpace(os.Getenv(envLeaderElectionNamespace))
	if ns != "" {
		return ns
	}
	return operatorName
}

// NamespacesToWatch get the array of namespaces to watch
func NamespacesToWatch() []string {
	val := os.Getenv(envNamespacesToWatch)
	if val == "" {
		return []string{}
	}
	namespaces := strings.Split(val, ",")
	for i, n := range namespaces {
		// cleanup
		namespaces[i] = strings.TrimSpace(n)
	}
	return namespaces
}

// GetWebHookCertDir returns the directory of the webhook certificates
func GetWebHookCertDir() string {
	def := filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs")
	return oputil.ValueOr(envWebHookCertificateDir, def)
}
