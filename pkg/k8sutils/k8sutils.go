package k8sutils

import (
	"context"

	certmanagerv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetCertificate(ctx context.Context, c client.Client, namespace, name string) (*certmanagerv1alpha2.Certificate, error) {
	cert := new(certmanagerv1alpha2.Certificate)
	err := c.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, cert)
	return cert, err
}

func EnsureCertificate(ctx context.Context, c client.Client, namespace, name string, transform func(cert *certmanagerv1alpha2.Certificate) *certmanagerv1alpha2.Certificate) (*certmanagerv1alpha2.Certificate, error) {
	cert, err := GetCertificate(ctx, c, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			cert.Namespace = namespace
			cert.Name = name
			if err := c.Create(ctx, transform(cert)); err != nil {
				return nil, err
			}
			return cert, nil
		}
		return nil, err
	}

	o := transform(cert.DeepCopy())
	if equality.Semantic.DeepEqual(cert, o) {
		return cert, nil
	}

	patch := client.MergeFrom(cert)
	if err := c.Patch(ctx, o, patch); err != nil {
		return nil, err
	}

	return o, nil
}

func GetSecret(ctx context.Context, c client.Client, namespace, name string) (*corev1.Secret, error) {
	s := new(corev1.Secret)
	err := c.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, s)
	return s, err
}
