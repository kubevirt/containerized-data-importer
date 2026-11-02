/*
Copyright 2026 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tlscryptowatch_test

import (
	"crypto/tls"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cryptowatch "kubevirt.io/containerized-data-importer/pkg/util/tls-crypto-watch"
)

var _ = Describe("TLS crypto watch", func() {
	Context("GroupsToCurves", func() {
		DescribeTable("should return no curves", func(groups []string) {
			Expect(cryptowatch.GroupsToCurves(groups)).To(BeEmpty())
		},
			Entry("for a nil list", nil),
			Entry("for an empty list", []string{}),
			Entry("for an empty group", []string{""}),
			Entry("for unknown group names", []string{"foo", "bar"}),
		)

		DescribeTable("should convert ", func(group string, curve tls.CurveID) {
			Expect(cryptowatch.GroupsToCurves([]string{group})).To(Equal([]tls.CurveID{curve}))
		},
			EntryDescription("%s to %s"),
			Entry(nil, cdiv1.TLSGroupX25519, tls.X25519),
			Entry(nil, cdiv1.TLSGroupSecP256r1, tls.CurveP256),
			Entry(nil, cdiv1.TLSGroupSecP384r1, tls.CurveP384),
			Entry(nil, cdiv1.TLSGroupSecP521r1, tls.CurveP521),
			Entry(nil, cdiv1.TLSGroupX25519MLKEM768, tls.X25519MLKEM768),
			Entry(nil, cdiv1.TLSGroupSecP256r1MLKEM768, tls.SecP256r1MLKEM768),
			Entry(nil, cdiv1.TLSGroupSecP384r1MLKEM1024, tls.SecP384r1MLKEM1024),
		)

		It("should keep known groups in order and skip unknown ones", func() {
			groups := []string{cdiv1.TLSGroupX25519, "foo", cdiv1.TLSGroupSecP256r1}
			Expect(cryptowatch.GroupsToCurves(groups)).To(Equal([]tls.CurveID{tls.X25519, tls.CurveP256}))
		})

		It("should map every group of the built-in profiles", func() {
			for profileType, spec := range cdiv1.TLSProfiles {
				Expect(cryptowatch.GroupsToCurves(spec.Groups)).To(HaveLen(len(spec.Groups)), "profile %s", profileType)
			}
		})
	})

	Context("GetTLSProfileSpec", func() {
		expectSpec := func(tlsSecurityProfile *cdiv1.TLSSecurityProfile, spec *cdiv1.TLSProfileSpec) {
			tlsProfileSpec := cryptowatch.GetTLSProfileSpec(tlsSecurityProfile)
			Expect(tlsProfileSpec.Ciphers).To(Equal(spec.Ciphers))
			Expect(tlsProfileSpec.MinTLSVersion).To(Equal(spec.MinTLSVersion))
			Expect(tlsProfileSpec.Groups).To(Equal(spec.Groups))
		}

		DescribeTable("should fall back to Intermediate for an incomplete profile", func(tlsSecurityProfile *cdiv1.TLSSecurityProfile) {
			expectSpec(tlsSecurityProfile, cdiv1.TLSProfiles[cdiv1.TLSProfileIntermediateType])
		},
			Entry("nil profile", nil),
			Entry("empty profile", &cdiv1.TLSSecurityProfile{}),
			Entry("Custom without a custom spec", &cdiv1.TLSSecurityProfile{Type: cdiv1.TLSProfileCustomType}),
		)

		DescribeTable("should return the built-in spec for a known type", func(tlsSecurityProfile *cdiv1.TLSSecurityProfile) {
			expectSpec(tlsSecurityProfile, cdiv1.TLSProfiles[tlsSecurityProfile.Type])
		},
			Entry("Old", &cdiv1.TLSSecurityProfile{Type: cdiv1.TLSProfileOldType, Old: &cdiv1.OldTLSProfile{}}),
			Entry("Modern", &cdiv1.TLSSecurityProfile{Type: cdiv1.TLSProfileModernType, Modern: &cdiv1.ModernTLSProfile{}}),
		)

		It("should return the custom spec for a Custom profile", func() {
			custom := cdiv1.TLSProfileSpec{
				Ciphers:       []string{"ECDHE-ECDSA-AES128-GCM-SHA256"},
				MinTLSVersion: cdiv1.VersionTLS11,
				Groups:        []string{cdiv1.TLSGroupSecP521r1},
			}
			p := &cdiv1.TLSSecurityProfile{
				Type:   cdiv1.TLSProfileCustomType,
				Custom: &cdiv1.CustomTLSProfile{TLSProfileSpec: custom},
			}
			expectSpec(p, &custom)
		})
	})
})
