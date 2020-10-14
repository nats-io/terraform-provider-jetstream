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

		if cmp.Equal(str.Subjects(), subjects) {
			return nil
		}

		return fmt.Errorf("expected %q got %q", subjects, str.Subjects())
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

func testStreamTemplateExist(t *testing.T, mgr *jsm.Manager, template string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		known, err := mgr.IsKnownStreamTemplate(template)
		if err != nil {
			return err
		}

		if !known {
			return fmt.Errorf("stream template %q does not exist", template)
		}

		return nil
	}
}

func testStreamTemplateDoesNotExist(t *testing.T, mgr *jsm.Manager, template string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		err := testStreamTemplateExist(t, mgr, template)
		if err == nil {
			return fmt.Errorf("expected stream template %q to not exist", template)
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
