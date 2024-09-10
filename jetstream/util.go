package jetstream

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var (
	connectMu sync.Mutex
)

func parseStreamKVEntryID(id string) (bucket string, key string, err error) {
	if !kvEntryIdRegex.MatchString(id) {
		return "", "", fmt.Errorf("invalid kv entry id %q", id)
	}

	matches := kvEntryIdRegex.FindStringSubmatch(id)
	return matches[1], matches[2], nil
}

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

func validateCompressionTypeString(i interface{}, p cty.Path) diag.Diagnostics {
	compression, _ := i.(string)
	compression = fmt.Sprintf("\"%s\"", compression)
	c := new(api.Compression)
	err := c.UnmarshalJSON([]byte(compression))
	if err != nil {
		detail := fmt.Sprintf("Invalid compression type for stream resource: '%s'", compression)
		return diag.Diagnostics{
			diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Invalid compression type",
				Detail:   detail,
			},
		}
	}

	return diag.Diagnostics{}
}

func validateRetentionTypeString() schema.SchemaValidateDiagFunc {
	return validation.ToDiagFunc(validation.StringInSlice([]string{"limits", "interest", "workqueue"}, false))
}

func validateDiscardPolicy() schema.SchemaValidateDiagFunc {
	return validation.ToDiagFunc(validation.StringInSlice([]string{"old", "new"}, false))
}

func validateStorageTypeString() schema.SchemaValidateDiagFunc {
	return validation.ToDiagFunc(validation.StringInSlice([]string{"file", "memory"}, false))
}

func streamSourceFromResourceData(d any) ([]*api.StreamSource, error) {
	ss := d.([]any)
	if len(ss) == 0 {
		return nil, fmt.Errorf("no data received")
	}

	var res []*api.StreamSource

	for _, sd := range ss {
		s, ok := sd.(map[string]any)
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

		exts := s["external"].([]any)
		if len(exts) > 0 {
			ext := exts[0].(map[string]any)
			source.External = &api.ExternalStream{
				ApiPrefix:     ext["api"].(string),
				DeliverPrefix: ext["deliver"].(string),
			}
		}

		transforms := s["subject_transform"].([]any)
		if len(transforms) > 0 {
			for _, transform := range transforms {
				st := transform.(map[string]any)
				source.SubjectTransforms = append(source.SubjectTransforms, api.SubjectTransformConfig{
					Source:      st["source"].(string),
					Destination: st["destination"].(string),
				})
			}
		}

		res = append(res, source)
	}

	return res, nil
}

func streamConfigFromResourceData(d *schema.ResourceData) (cfg api.StreamConfig, err error) {
	var retention api.RetentionPolicy
	var storage api.StorageType
	var discard api.DiscardPolicy

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

	switch d.Get("discard").(string) {
	case "new":
		discard = api.DiscardNew
	case "old":
		discard = api.DiscardOld
	}

	subs := d.Get("subjects").([]any)
	var subjects = make([]string, len(subs))
	for i, sub := range subs {
		subjects[i] = sub.(string)
	}

	var subjectTransforms *api.SubjectTransformConfig
	transforms := d.Get("subject_transform").([]any)

	if len(transforms) == 1 {
		for _, transform := range transforms {
			st := transform.(map[string]any)
			subjectTransforms = &api.SubjectTransformConfig{
				Source:      st["source"].(string),
				Destination: st["destination"].(string),
			}
		}
	}

	var placement *api.Placement
	c, ok := d.GetOk("placement_cluster")
	if ok {
		placement = &api.Placement{Cluster: c.(string)}
		pt, ok := d.GetOk("placement_tags")
		if ok {
			ts := pt.([]any)
			var tags = make([]string, len(ts))
			for i, tag := range ts {
				tags[i] = tag.(string)
			}
			placement.Tags = tags
		}
	}

	stream := api.StreamConfig{
		Name:             d.Get("name").(string),
		Subjects:         subjects,
		Retention:        retention,
		Discard:          discard,
		DiscardNewPer:    d.Get("discard_new_per_subject").(bool),
		MaxConsumers:     d.Get("max_consumers").(int),
		MaxMsgs:          int64(d.Get("max_msgs").(int)),
		MaxBytes:         int64(d.Get("max_bytes").(int)),
		MaxAge:           time.Second * time.Duration(d.Get("max_age").(int)),
		Duplicates:       time.Second * time.Duration(d.Get("duplicate_window").(int)),
		MaxMsgSize:       int32(d.Get("max_msg_size").(int)),
		Storage:          storage,
		Replicas:         d.Get("replicas").(int),
		NoAck:            !d.Get("ack").(bool),
		AllowDirect:      d.Get("allow_direct").(bool),
		DenyDelete:       d.Get("deny_delete").(bool),
		DenyPurge:        d.Get("deny_purge").(bool),
		RollupAllowed:    d.Get("allow_rollup_hdrs").(bool),
		Placement:        placement,
		SubjectTransform: subjectTransforms,
	}

	repubSrc := d.Get("republish_source").(string)
	repubDest := d.Get("republish_destination").(string)
	repubHdrs := d.Get("republish_headers_only").(bool)
	if repubSrc != "" {
		stream.RePublish = &api.RePublish{
			Source:      repubSrc,
			Destination: repubDest,
			HeadersOnly: repubHdrs,
		}
	}

	if description, ok := d.GetOk("description"); ok {
		stream.Description = description.(string)
	}

	if limit := d.Get("max_msgs_per_subject"); limit != nil {
		stream.MaxMsgsPer = int64(limit.(int))
	}

	compression := d.Get("compression")
	switch compression {
	case "none":
		stream.Compression = api.NoCompression
	case "s2":
		stream.Compression = api.S2Compression
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
		mirrorDirect, ok := d.GetOk("mirror_direct")
		if ok {
			stream.MirrorDirect = mirrorDirect.(bool)
		}
	}

	ss, ok := d.GetOk("source")
	if ok {
		sources, err := streamSourceFromResourceData(ss)
		if err != nil {
			return api.StreamConfig{}, err
		}
		stream.Sources = sources
	}

	if stream.Mirror != nil {
		if len(stream.Sources) > 0 {
			return api.StreamConfig{}, fmt.Errorf("only one of sources and mirror may be specified")
		}
	}

	if len(stream.Subjects) == 0 && stream.Mirror == nil && len(stream.Sources) == 0 {
		return api.StreamConfig{}, fmt.Errorf("subjects are required for streams without mirrors or sources")
	}

	m, ok := d.GetOk("metadata")
	if ok {
		mt, ok := m.(map[string]any)
		if ok {
			meta := map[string]string{}
			for k, v := range mt {
				meta[k] = v.(string)
			}
			stream.Metadata = jsm.FilterServerMetadata(meta)
		} else {
			return api.StreamConfig{}, fmt.Errorf("invalid metadata")
		}
	}

	max_ack_pending, ok := d.GetOk("max_ack_pending")
	if ok {
		stream.ConsumerLimits.MaxAckPending = max_ack_pending.(int)
	}

	inactive_threshold, ok := d.GetOk("inactive_threshold")
	if ok {
		stream.ConsumerLimits.InactiveThreshold = time.Second * time.Duration(inactive_threshold.(int))
	}

	ok, errs := stream.Validate(new(SchemaValidator))
	if !ok {
		return api.StreamConfig{}, errors.New(strings.Join(errs, ", "))
	}

	return stream, nil
}

func newTempPEMFile(pemContents string) (filename string, cleanup func(), err error) {
	file, err := os.CreateTemp("", "*.pem")
	if err != nil {
		return
	}
	defer file.Close()

	_, err = file.WriteString(pemContents)
	if err != nil {
		return
	}

	cleanup = func() {
		_ = os.Remove(filename)
	}

	return file.Name(), cleanup, err
}

func wipeSlice(buf []byte) {
	for i := range buf {
		buf[i] = 'x'
	}
}

type connectProperties struct {
	creds           string
	credData        []byte
	servers         string
	user            string
	pass            string
	nkey            string
	caFile          string
	certFile        string
	keyFile         string
	cleanupCaFile   func()
	cleanupCertFile func()
	cleanupKeyFile  func()
}

func getConnectProperties(d *schema.ResourceData) (*connectProperties, error) {
	connectMu.Lock()
	defer connectMu.Unlock()

	p := connectProperties{
		creds:           "",
		credData:        nil,
		servers:         "",
		user:            "",
		pass:            "",
		nkey:            "",
		caFile:          "",
		certFile:        "",
		keyFile:         "",
		cleanupCaFile:   nil,
		cleanupCertFile: nil,
		cleanupKeyFile:  nil,
	}

	s := d.Get("credentials")
	if s != nil {
		p.creds = s.(string)
	}

	s = d.Get("credential_data")
	if s != nil {
		p.credData = []byte(s.(string))
	}

	s = d.Get("servers")
	if s != nil {
		p.servers = s.(string)
	}

	s = d.Get("user")
	if s != nil {
		p.user = s.(string)
	}

	s = d.Get("password")
	if s != nil {
		p.pass = s.(string)
	}

	s = d.Get("nkey")
	if s != nil {
		p.nkey = s.(string)
	}

	s = d.Get("tls")
	if s != nil {
		set := s.(*schema.Set)

		for _, v := range set.List() {
			m := v.(map[string]any)

			for k, v := range m {
				switch {
				case k == "ca_file" && len(v.(string)) > 0:
					p.caFile = v.(string)

				case k == "ca_file_data" && len(v.(string)) > 0:
					file, cleanup, err := newTempPEMFile(v.(string))
					if err != nil {
						return nil, err
					}

					p.cleanupCaFile = cleanup
					p.caFile = file

				case k == "cert_file" && len(v.(string)) > 0:
					p.certFile = v.(string)

				case k == "cert_file_data" && len(v.(string)) > 0:
					file, cleanup, err := newTempPEMFile(v.(string))
					if err != nil {
						return nil, err
					}

					p.cleanupCertFile = cleanup
					p.certFile = file

				case k == "key_file" && len(v.(string)) > 0:
					p.keyFile = v.(string)

				case k == "key_file_data" && len(v.(string)) > 0:
					file, cleanup, err := newTempPEMFile(v.(string))
					if err != nil {
						return nil, err
					}

					p.cleanupKeyFile = cleanup
					p.keyFile = file
				}
			}
		}
	}

	return &p, nil
}

func connectMgr(d *schema.ResourceData) (any, error) {
	return func() (*nats.Conn, *jsm.Manager, error) {
		props, err := getConnectProperties(d)
		if err != nil {
			return nil, nil, err
		}

		var opts []nats.Option

		switch {
		case props.creds != "":
			opts = append(opts, nats.UserCredentials(props.creds))

		case len(props.credData) > 0:
			defer wipeSlice(props.credData)

			userCB := func() (string, error) {
				return jwt.ParseDecoratedJWT(props.credData)
			}

			sigCB := func(nonce []byte) ([]byte, error) {
				kp, err := jwt.ParseDecoratedNKey(props.credData)
				if err != nil {
					return nil, err
				}
				defer kp.Wipe()

				return kp.Sign(nonce)
			}

			opts = append(opts, nats.UserJWT(userCB, sigCB))
		}

		switch {
		case props.user != "" && props.pass != "":
			opts = append(opts, nats.UserInfo(props.user, props.pass))
		case props.user != "":
			opts = append(opts, nats.Token(props.user))
		case props.nkey != "":
			nko, err := nats.NkeyOptionFromSeed(props.nkey)
			if err != nil {
				return nil, nil, err
			}

			opts = append(opts, nko)
		}

		if len(props.caFile) > 0 {
			if props.cleanupCaFile != nil {
				defer props.cleanupCaFile()
			}

			opts = append(opts, nats.RootCAs(props.caFile))
		}

		if len(props.certFile) > 0 || len(props.keyFile) > 0 {
			if props.cleanupCertFile != nil {
				defer props.cleanupCertFile()
			}

			if props.cleanupKeyFile != nil {
				defer props.cleanupKeyFile()
			}

			if len(props.certFile) == 0 || len(props.keyFile) == 0 {
				return nil, nil, fmt.Errorf("cert_file and key_file depend on each other, if one is provided the other one must be provided as well")
			}

			opts = append(opts, nats.ClientCert(props.certFile, props.keyFile))
		}

		nc, err := nats.Connect(props.servers, opts...)
		if err != nil {
			return nil, nil, err
		}

		mgr, err := jsm.New(nc, jsm.WithAPIValidation(new(SchemaValidator)), jsm.WithPedanticRequests())
		if err != nil {
			return nil, nil, err
		}

		apiLevel, err := mgr.MetaApiLevel(true)
		if err != nil {
			return nil, nil, err
		}

		if apiLevel < 1 {
			return nil, nil, fmt.Errorf("unsupported api level: %d. Requires NATS Server 2.11 or newer", apiLevel)
		}

		return nc, mgr, err
	}, nil
}
