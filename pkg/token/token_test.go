/*
 * This file is part of the CDI project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2019 Red Hat, Inc.
 *
 */
package token

import (
	"crypto/rand"
	"crypto/rsa"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/tests/reporters"
)

func generateTestKey() (*rsa.PrivateKey, error) {
	apiKeyPair, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return apiKeyPair, nil
}

func TestToken(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Token Suite", reporters.NewReporters())
}

var _ = Describe("Token test", func() {
	It("Token", func() {
		issuer := "issuer"

		key, err := generateTestKey()
		Expect(err).ToNot(HaveOccurred())

		tokenData := &Payload{
			Operation: OperationUpload,
			Name:      "fakepvc",
			Namespace: "fakenamespace",
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "persistentvolumeclaims",
			},
		}

		g := NewGenerator(issuer, key, 5*time.Minute)

		signedToken, err := g.Generate(tokenData)
		Expect(err).ToNot(HaveOccurred())

		validator := NewValidator(issuer, &key.PublicKey, 0)

		payload, err := validator.Validate(signedToken)
		Expect(err).ToNot(HaveOccurred())
		Expect(reflect.DeepEqual(tokenData, payload)).To(BeTrue())
	})

	It("Token timeout", func() {
		issuer := "issuer"

		key, err := generateTestKey()
		Expect(err).ToNot(HaveOccurred())

		tokenData := &Payload{
			Operation: OperationUpload,
			Name:      "fakepvc",
			Namespace: "fakenamespace",
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "persistentvolumeclaims",
			},
		}

		g := NewGenerator(issuer, key, 200*time.Millisecond)

		signedToken, err := g.Generate(tokenData)
		Expect(err).ToNot(HaveOccurred())

		validator := NewValidator(issuer, &key.PublicKey, 0)

		time.Sleep(time.Second)

		_, err = validator.Validate(signedToken)
		Expect(err).To(HaveOccurred())
	})

	It("Wrong issuer", func() {
		key, err := generateTestKey()
		Expect(err).ToNot(HaveOccurred())

		tokenData := &Payload{
			Operation: OperationUpload,
			Name:      "fakepvc",
			Namespace: "fakenamespace",
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "persistentvolumeclaims",
			},
		}

		g := NewGenerator("foo", key, 5*time.Minute)

		signedToken, err := g.Generate(tokenData)
		Expect(err).ToNot(HaveOccurred())

		validator := NewValidator("bar", &key.PublicKey, 0)

		_, err = validator.Validate(signedToken)
		Expect(err).To(HaveOccurred())
	})

	It("Bad key", func() {
		issuer := "issuer"

		key, err := generateTestKey()
		Expect(err).ToNot(HaveOccurred())

		key2, err := generateTestKey()
		Expect(err).ToNot(HaveOccurred())

		tokenData := &Payload{
			Operation: OperationUpload,
			Name:      "fakepvc",
			Namespace: "fakenamespace",
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "persistentvolumeclaims",
			},
		}

		g := NewGenerator(issuer, key, 5*time.Minute)

		signedToken, err := g.Generate(tokenData)
		Expect(err).ToNot(HaveOccurred())

		validator := NewValidator(issuer, &key2.PublicKey, 0)

		time.Sleep(time.Second)

		_, err = validator.Validate(signedToken)
		Expect(err).To(HaveOccurred())
	})
})
