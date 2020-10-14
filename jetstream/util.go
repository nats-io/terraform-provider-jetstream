package jetstream

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jwt"
	"github.com/nats-io/nats.go"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func parseStreamID(id string) (string, error) {
	if !streamIdRegex.MatchString(id) {
		return "", fmt.Errorf("invalid stream id %q", id)
	}

	matches := streamIdRegex.FindStringSubmatch(id)

	return matches[1], nil
}

func parseConsumerID(id string) (stream string, consumer string, err error) {
	if !consumerIdRegex.MatchString(id) {
		return "", "", fmt.Errorf("invalid consumer id %q", id)
	}

	matches := consumerIdRegex.FindStringSubmatch(id)

	return matches[1], matches[2], nil
}

func parseStreamTemplateID(id string) (string, error) {
	if !streamTemplateIdRegex.MatchString(id) {
		return "", fmt.Errorf("invalid stream template id %q", id)
	}

	matches := streamTemplateIdRegex.FindStringSubmatch(id)

	return matches[1], nil
}

func validateRetentionTypeString() schema.SchemaValidateFunc {
	return validation.StringInSlice([]string{"limits", "interest", "workqueue"}, false)
}

func validateStorageTypeString() schema.SchemaValidateFunc {
	return validation.StringInSlice([]string{"file", "memory"}, false)
}

func streamConfigFromResourceData(d *schema.ResourceData) (cfg api.StreamConfig, err error) {
	var retention api.RetentionPolicy
	var storage api.StorageType

	switch d.Get("retention").(string) {
	case "limits":
		retention = api.LimitsPolicy
	case "interest":
		retention = api.InterestPolicy
	case "workqueue":
		retention = api.WorkQueuePolicy
	}

	switch d.Get("storage").(string) {
	case "file":
		storage = api.FileStorage
	case "memory":
		storage = api.MemoryStorage
	}

	subs := d.Get("subjects").([]interface{})
	var subjects = make([]string, len(subs))
	for i, sub := range subs {
		subjects[i] = sub.(string)
	}

	return api.StreamConfig{
		Name:         d.Get("name").(string),
		Subjects:     subjects,
		Retention:    retention,
		MaxConsumers: d.Get("max_consumers").(int),
		MaxMsgs:      int64(d.Get("max_msgs").(int)),
		MaxBytes:     int64(d.Get("max_bytes").(int)),
		MaxAge:       time.Second * time.Duration(d.Get("max_age").(int)),
		Duplicates:   time.Second * time.Duration(d.Get("duplicate_window").(int)),
		MaxMsgSize:   int32(d.Get("max_msg_size").(int)),
		Storage:      storage,
		Replicas:     d.Get("replicas").(int),
		NoAck:        !d.Get("ack").(bool),
	}, nil
}

func wipeSlice(buf []byte) {
	for i := range buf {
		buf[i] = 'x'
	}
}

type conn struct {
	nc  *nats.Conn
	mgr *jsm.Manager
}

func connectMgr(d *schema.ResourceData) (interface{}, error) {
	var (
		creds    string
		credData []byte
		servers  string
	)

	s := d.Get("credentials")
	if s != nil {
		creds = s.(string)
	}

	s = d.Get("credential_data")
	if s != nil {
		credData = []byte(s.(string))
	}

	s = d.Get("servers")
	if s != nil {
		servers = s.(string)
	}

	var opts []nats.Option

	switch {
	case creds != "":
		opts = append(opts, nats.UserCredentials(creds))

	case len(credData) > 0:
		defer wipeSlice(credData)

		userCB := func() (string, error) {
			return jwt.ParseDecoratedJWT(credData)
		}

		sigCB := func(nonce []byte) ([]byte, error) {
			kp, err := jwt.ParseDecoratedNKey(credData)
			if err != nil {
				return nil, err
			}
			defer kp.Wipe()

			return kp.Sign(nonce)
		}

		opts = append(opts, nats.UserJWT(userCB, sigCB))

	}

	nc, err := nats.Connect(servers, opts...)
	if err != nil {
		return nil, err
	}

	mgr, err := jsm.New(nc, jsm.WithAPIValidation(new(SchemaValidator)))
	if err != nil {
		return nil, err
	}

	return &conn{nc, mgr}, err
}
