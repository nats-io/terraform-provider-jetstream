package jetstream

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/nats-io/jsm.go"
)

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
