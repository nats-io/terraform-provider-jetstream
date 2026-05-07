// Copyright 2025 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jetstream

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

func testStreamHasMetadata(t *testing.T, mgr *jsm.Manager, stream string, metadata map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if cmp.Equal(jsm.FilterServerMetadata(str.Metadata()), metadata) {
			return nil
		}

		return fmt.Errorf("expected %v got %v", metadata, str.Metadata())
	}
}

func testConsumerHasMetadata(t *testing.T, mgr *jsm.Manager, stream string, consumer string, metadata map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		cons, err := mgr.LoadConsumer(stream, consumer)
		if err != nil {
			return err
		}

		if cmp.Equal(jsm.FilterServerMetadata(cons.Metadata()), metadata) {
			return nil
		}

		return fmt.Errorf("expected %v got %v", metadata, cons.Metadata())
	}
}

func testConsumerHasFilterSubject(t *testing.T, mgr *jsm.Manager, stream string, consumer string, subject string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		cons, err := mgr.LoadConsumer(stream, consumer)
		if err != nil {
			return err
		}
		if cons.FilterSubject() == subject {
			return nil
		}

		return fmt.Errorf("expected %q got %q", subject, cons.FilterSubject())
	}
}
func testConsumerHasFilterSubjects(t *testing.T, mgr *jsm.Manager, stream string, consumer string, subjects []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		cons, err := mgr.LoadConsumer(stream, consumer)
		if err != nil {
			return err
		}
		if len(cons.FilterSubjects()) == 0 && len(subjects) == 0 {
			return nil
		}
		if cmp.Equal(cons.FilterSubjects(), subjects) {
			return nil
		}

		return fmt.Errorf("expected %q got %q", subjects, cons.FilterSubjects())
	}
}

func testStreamHasFirstSeq(t *testing.T, mgr *jsm.Manager, stream string, expected uint64) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}
		if got := str.Configuration().FirstSeq; got != expected {
			return fmt.Errorf("expected first_seq %d got %d", expected, got)
		}
		return nil
	}
}

func testStreamHasPersistMode(t *testing.T, mgr *jsm.Manager, stream string, expected api.PersistModeType) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}
		if got := str.Configuration().PersistMode; got != expected {
			return fmt.Errorf("expected persist_mode %v got %v", expected, got)
		}
		return nil
	}
}

func testStreamAllowsBatched(t *testing.T, mgr *jsm.Manager, stream string, expected bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}
		if got := str.Configuration().AllowBatchPublish; got != expected {
			return fmt.Errorf("expected allow_batched %v got %v", expected, got)
		}
		return nil
	}
}

func testStreamSourceHasConsumer(t *testing.T, mgr *jsm.Manager, stream string, sourceName string, name string, deliverSubject string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}
		for _, src := range str.Sources() {
			if src.Name != sourceName {
				continue
			}
			if src.Consumer == nil {
				return fmt.Errorf("source %q has no consumer set", sourceName)
			}
			if src.Consumer.Name != name || src.Consumer.DeliverSubject != deliverSubject {
				return fmt.Errorf("source %q consumer = {%q, %q}, want {%q, %q}", sourceName, src.Consumer.Name, src.Consumer.DeliverSubject, name, deliverSubject)
			}
			return nil
		}
		return fmt.Errorf("stream %q has no source named %q", stream, sourceName)
	}
}

func testConsumerHasAckPolicy(t *testing.T, mgr *jsm.Manager, stream string, consumer string, policy api.AckPolicy) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		cons, err := mgr.LoadConsumer(stream, consumer)
		if err != nil {
			return err
		}
		if cons.AckPolicy() == policy {
			return nil
		}

		return fmt.Errorf("expected ack policy %q got %q", policy, cons.AckPolicy())
	}
}

func testStreamHasSubjects(t *testing.T, mgr *jsm.Manager, stream string, subjects []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if len(str.Subjects()) == 0 && len(subjects) == 0 {
			return nil
		}
		if cmp.Equal(str.Subjects(), subjects) {
			return nil
		}

		return fmt.Errorf("expected %q got %q", subjects, str.Subjects())
	}
}

func testStreamIsMirrorTransformed(t *testing.T, mgr *jsm.Manager, stream string, transforms ...api.SubjectTransformConfig) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if !str.IsMirror() {
			return fmt.Errorf("stream is not a mirror")
		}

		mirror := str.Mirror()

		for i, trans := range transforms {
			if len(mirror.SubjectTransforms) == 0 || len(mirror.SubjectTransforms) < i {
				return fmt.Errorf("transform %v does not match %v", mirror.SubjectTransforms, trans)
			}
			if mirror.SubjectTransforms[i].Source != trans.Source {
				return fmt.Errorf("transform %v does not match %v", mirror.SubjectTransforms[i], trans)
			}
			if mirror.SubjectTransforms[i].Destination != trans.Destination {
				return fmt.Errorf("transform %v does not match %v", mirror.SubjectTransforms[i], trans)
			}
		}

		return nil
	}
}

func testStreamIsSourceTransformed(t *testing.T, mgr *jsm.Manager, stream string, sourceName string, transforms ...api.SubjectTransformConfig) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if !str.IsSourced() {
			return fmt.Errorf("stream is not sourced")
		}

		var source *api.StreamSource

		for _, s := range str.Sources() {
			if s.Name == sourceName {
				source = s
				break
			}
		}

		if source == nil {
			return fmt.Errorf("source not found")
		}

		for i, trans := range transforms {
			if len(source.SubjectTransforms) == 0 || len(source.SubjectTransforms) < i {
				return fmt.Errorf("transform %v does not match %v", source.SubjectTransforms, trans)
			}
			if source.SubjectTransforms[i].Source != trans.Source {
				return fmt.Errorf("transform %v does not match %v", source.SubjectTransforms[i], trans)
			}
			if source.SubjectTransforms[i].Destination != trans.Destination {
				return fmt.Errorf("transform %v does not match %v", source.SubjectTransforms[i], trans)
			}
		}

		return nil
	}
}

func testStreamIsSourceOf(t *testing.T, mgr *jsm.Manager, stream string, sources []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if !str.IsSourced() {
			return fmt.Errorf("stream is not sourced")
		}

		for _, name := range sources {
			found := false
			for _, source := range str.Sources() {
				if source.Name == name {
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("stream is not a source of %s", name)
			}
		}

		return nil
	}
}

func testStreamIsMirrorOf(t *testing.T, mgr *jsm.Manager, stream string, mirror string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if !str.IsMirror() {
			return fmt.Errorf("stream is not mirrored")
		}
		if str.Mirror().Name != mirror {
			return fmt.Errorf("stream is mirror of %q", str.Mirror().Name)
		}

		return nil
	}
}

func testStreamAllowsTTLs(t *testing.T, mgr *jsm.Manager, stream string, markerTTL time.Duration) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if !str.AllowMsgTTL() {
			return fmt.Errorf("stream does not allow TTLs")
		}
		if markerTTL == 0 {
			return nil
		}

		if str.SubjectDeleteMarkerTTL() != markerTTL {
			return fmt.Errorf("stream marker ttl %v is not equal to %v", str.SubjectDeleteMarkerTTL(), markerTTL)
		}

		return nil
	}
}

func testStreamIsCompressed(t *testing.T, mgr *jsm.Manager, stream string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if str.Compression() != api.S2Compression {
			return fmt.Errorf("stream is uncompressed")
		}

		return nil
	}
}

func testStreamIsUnCompressed(t *testing.T, mgr *jsm.Manager, stream string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if str.Compression() != api.NoCompression {
			return fmt.Errorf("stream is compressed")
		}

		return nil
	}
}

func testStreamExist(t *testing.T, mgr *jsm.Manager, stream string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		known, err := mgr.IsKnownStream(stream)
		if err != nil {
			return err
		}

		if !known {
			return fmt.Errorf("stream %q does not exist", stream)
		}

		return nil
	}
}

func testStreamDoesNotExist(t *testing.T, mgr *jsm.Manager, stream string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		err := testStreamExist(t, mgr, stream)
		if err == nil {
			return fmt.Errorf("expected stream %q to not exist", stream)
		}

		return nil
	}
}

func testConsumerExist(t *testing.T, mgr *jsm.Manager, stream string, consumer string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		known, err := mgr.IsKnownConsumer(stream, consumer)
		if err != nil {
			return err
		}

		if !known {
			return fmt.Errorf("expected %q > %q to exist", stream, consumer)
		}

		return nil
	}
}

func testConsumerDoesNotExist(t *testing.T, mgr *jsm.Manager, stream string, consumer string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		err := testConsumerExist(t, mgr, stream, consumer)
		if err == nil {
			return fmt.Errorf("expected consumer %q > %q to not exist", stream, consumer)
		}

		return nil
	}
}

func testStreamIsTransformed(t *testing.T, mgr *jsm.Manager, stream string, transform api.SubjectTransformConfig) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if *str.Configuration().SubjectTransform != transform {
			return fmt.Errorf("subject transform %v does not match %v", *str.Configuration().SubjectTransform, transform)
		}

		return nil
	}
}
