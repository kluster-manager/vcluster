package platform

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	managementv1 "github.com/loft-sh/api/v4/pkg/apis/management/v1"
	storagev1 "github.com/loft-sh/api/v4/pkg/apis/storage/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const DefaultPlatformSecretName = "vcluster-platform-api-key"

func ApplyPlatformSecret(ctx context.Context, client Client, kubeClient kubernetes.Interface, importName, namespace, project string) error {
	managementClient, err := client.Management()
	if err != nil {
		return fmt.Errorf("create management client: %w", err)
	}

	platformConfig := client.Config().Platform

	// is the access key still valid?
	if platformConfig.VirtualClusterAccessKey != "" {
		selfCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		self, err := managementClient.Loft().ManagementV1().Selves().Create(selfCtx, &managementv1.Self{
			Spec: managementv1.SelfSpec{
				AccessKey: platformConfig.VirtualClusterAccessKey,
			},
		}, metav1.CreateOptions{})
		cancel()
		if err != nil || self.Status.Subject != client.Self().Status.Subject {
			platformConfig.VirtualClusterAccessKey = ""
		}
	}

	// check if we need to create a virtual cluster access key
	if platformConfig.VirtualClusterAccessKey == "" {
		user := ""
		team := ""
		if client.Self().Status.User != nil {
			user = client.Self().Status.User.Name
		}
		if client.Self().Status.Team != nil {
			team = client.Self().Status.Team.Name
		}

		accessKey, err := managementClient.Loft().ManagementV1().OwnedAccessKeys().Create(ctx, &managementv1.OwnedAccessKey{
			Spec: managementv1.OwnedAccessKeySpec{
				AccessKeySpec: storagev1.AccessKeySpec{
					DisplayName: "vCluster CLI Activation Key",
					User:        user,
					Team:        team,
					Scope: &storagev1.AccessKeyScope{
						Roles: []storagev1.AccessKeyScopeRole{
							{
								Role: storagev1.AccessKeyScopeRoleVCluster,
							},
						},
					},
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create owned access key: %w", err)
		}

		platformConfig.VirtualClusterAccessKey = accessKey.Spec.Key
		client.Config().Platform = platformConfig
		if err := client.Save(); err != nil {
			return fmt.Errorf("save vCluster platform config: %w", err)
		}
	}

	// build secret payload
	payload := map[string][]byte{
		"accessKey": []byte(client.Config().Platform.VirtualClusterAccessKey),
		"host":      []byte(strings.TrimPrefix(client.Config().Platform.Host, "https://")),
		"insecure":  []byte(strconv.FormatBool(client.Config().Platform.Insecure)),
	}
	if project != "" {
		payload["project"] = []byte(project)
	}
	if importName != "" {
		payload["name"] = []byte(importName)
	}

	// check if secret already exists
	keySecret, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, DefaultPlatformSecretName, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("error getting platform secret %s/%s: %w", namespace, DefaultPlatformSecretName, err)
	} else if kerrors.IsNotFound(err) {
		_, err = kubeClient.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      DefaultPlatformSecretName,
				Namespace: namespace,
			},
			Data: payload,
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating platform secret %s/%s: %w", namespace, DefaultPlatformSecretName, err)
		}

		return nil
	} else if reflect.DeepEqual(keySecret.Data, payload) {
		// no update needed, just return
		return nil
	}

	// create the patch
	patch := ctrlclient.MergeFrom(keySecret.DeepCopy())
	keySecret.Data = payload
	patchBytes, err := patch.Data(keySecret)
	if err != nil {
		return fmt.Errorf("error creating patch for platform secret %s/%s: %w", namespace, DefaultPlatformSecretName, err)
	}

	// patch the secret
	_, err = kubeClient.CoreV1().Secrets(namespace).Patch(ctx, keySecret.Name, patch.Type(), patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error patching platform secret %s/%s: %w", namespace, DefaultPlatformSecretName, err)
	}

	return nil
}
