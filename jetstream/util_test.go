package jetstream

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

func testStreamHasMetadata(t *testing.T, mgr *jsm.Manager, stream string, metadata map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := mgr.LoadStream(stream)
		if err != nil {
			return err
		}

		if cmp.Equal(str.Metadata(), metadata) {
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

		if cmp.Equal(cons.Metadata(), metadata) {
			return nil
		}

		return fmt.Errorf("expected %v got %v", metadata, cons.Metadata())
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

//func testStreamIsTransformed

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
