package jetstream

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/nats-io/jsm.go"
)

func testStreamHasSubjects(t *testing.T, stream string, subjects []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		str, err := jsm.LoadStream(stream)
		if err != nil {
			return err
		}

		if cmp.Equal(str.Subjects(), subjects) {
			return nil
		}

		return fmt.Errorf("expected %q got %q", subjects, str.Subjects())
	}
}

func testStreamExist(t *testing.T, stream string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		known, err := jsm.IsKnownStream(stream)
		if err != nil {
			return err
		}

		if !known {
			return fmt.Errorf("stream %q does not exist on %q", stream, jsm.Connection().ConnectedUrl())
		}

		return nil
	}
}

func testStreamTemplateExist(t *testing.T, template string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		known, err := jsm.IsKnownStreamTemplate(template)
		if err != nil {
			return err
		}

		if !known {
			return fmt.Errorf("stream template %q does not exist on %q", template, jsm.Connection().ConnectedUrl())
		}

		return nil
	}
}

func testStreamTemplateDoesNotExist(t *testing.T, template string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		err := testStreamTemplateExist(t, template)
		if err == nil {
			return fmt.Errorf("expected stream template %q to not exist on %q", template, jsm.Connection().ConnectedUrl())
		}

		return nil
	}
}

func testStreamDoesNotExist(t *testing.T, stream string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		err := testStreamExist(t, stream)
		if err == nil {
			return fmt.Errorf("expected stream %q to not exist on %q", stream, jsm.Connection().ConnectedUrl())
		}

		return nil
	}
}

func testConsumerExist(t *testing.T, stream string, consumer string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		known, err := jsm.IsKnownConsumer(stream, consumer)
		if err != nil {
			return err
		}

		if !known {
			return fmt.Errorf("expected %q > %q to exist on %q", stream, consumer, jsm.Connection().ConnectedUrl())
		}

		return nil
	}
}

func testConsumerDoesNotExist(t *testing.T, stream string, consumer string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		err := testConsumerExist(t, stream, consumer)
		if err == nil {
			return fmt.Errorf("expected consumer %q > %q to not exist on %q", stream, consumer, jsm.Connection().ConnectedUrl())
		}

		return nil
	}
}
