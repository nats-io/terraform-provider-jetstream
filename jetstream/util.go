package jetstream

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jwt"
	"github.com/nats-io/nats.go"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func parseStreamKVID(id string) (string, error) {
	if !kvIdRegex.MatchString(id) {
		return "", fmt.Errorf("invalid kv bucket id %q", id)
	}

	matches := kvIdRegex.FindStringSubmatch(id)
	return matches[1], nil
}

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

func streamSourceFromResourceData(d interface{}) ([]*api.StreamSource, error) {
	ss := d.([]interface{})
	if len(ss) == 0 {
		return nil, fmt.Errorf("no data received")
	}

	var res []*api.StreamSource

	for _, sd := range ss {
		s, ok := sd.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid hashmap received")
		}

		source := &api.StreamSource{
			Name:          s["name"].(string),
			OptStartSeq:   uint64(s["start_seq"].(int)),
			FilterSubject: s["filter_subject"].(string),
		}

		st := s["start_time"].(string)
		if st != "" {
			ts, err := time.Parse(time.RFC3339, st)
			if err != nil {
				return nil, err
			}
			source.OptStartTime = &ts
		}

		exts := s["external"].([]interface{})
		if len(exts) > 0 {
			ext := exts[0].(map[string]interface{})
			source.External = &api.ExternalStream{
				ApiPrefix:     ext["api"].(string),
				DeliverPrefix: ext["deliver"].(string),
			}
		}

		res = append(res, source)
	}

	return res, nil
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

	var placement *api.Placement
	c, ok := d.GetOk("placement_cluster")
	if ok {
		placement = &api.Placement{Cluster: c.(string)}
		pt, ok := d.GetOk("placement_tags")
		if ok {
			ts := pt.([]interface{})
			var tags = make([]string, len(ts))
			for i, tag := range ts {
				tags[i] = tag.(string)
			}
			placement.Tags = tags
		}
	}

	stream := api.StreamConfig{
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
		Placement:    placement,
	}

	if description, ok := d.GetOk("description"); ok {
		stream.Description = description.(string)
	}

	if limit := d.Get("max_msgs_per_subject"); limit != nil {
		stream.MaxMsgsPer = int64(limit.(int))
	}

	mirror, ok := d.GetOk("mirror")
	if ok {
		sources, err := streamSourceFromResourceData(mirror)
		if err != nil {
			return api.StreamConfig{}, err
		}
		if len(sources) != 1 {
			return api.StreamConfig{}, fmt.Errorf("expected exactly one mirror source")
		}
		stream.Mirror = sources[0]
	}

	ss, ok := d.GetOk("source")
	if ok {
		sources, err := streamSourceFromResourceData(ss)
		if err != nil {
			return api.StreamConfig{}, err
		}
		stream.Sources = sources
	}

	if stream.Mirror != nil && len(stream.Sources) > 0 {
		return api.StreamConfig{}, fmt.Errorf("only one of sources and mirror may be specified")
	}

	if len(stream.Subjects) == 0 && stream.Mirror == nil && len(stream.Sources) == 0 {
		return api.StreamConfig{}, fmt.Errorf("subjects are required for streams without mirrors or sources")
	}

	ok, errs := stream.Validate(new(SchemaValidator))
	if !ok {
		return api.StreamConfig{}, fmt.Errorf(strings.Join(errs, ", "))
	}

	return stream, nil
}

func wipeSlice(buf []byte) {
	for i := range buf {
		buf[i] = 'x'
	}
}

func connectMgr(d *schema.ResourceData) (interface{}, error) {
	return func() (*nats.Conn, *jsm.Manager, error) {
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
			return nil, nil, err
		}

		mgr, err := jsm.New(nc, jsm.WithAPIValidation(new(SchemaValidator)))
		if err != nil {
			return nil, nil, err
		}

		return nc, mgr, err
	}, nil
}
