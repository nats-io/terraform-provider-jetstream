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
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func resourceKVBucket() *schema.Resource {
	return &schema.Resource{
		Create: resourceKVBucketCreate,
		Read:   resourceKVBucketRead,
		Update: resourceKVBucketUpdate,
		Delete: resourceKVBucketDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Description: "The name of the Bucket",
				Required:    true,
				ForceNew:    true,
			},
			"description": {
				Type:        schema.TypeString,
				Description: "Contains additional information about this bucket",
				Optional:    true,
				ForceNew:    false,
			},
			"history": {
				Type:         schema.TypeInt,
				Description:  "How many historical values to keep",
				Default:      5,
				Optional:     true,
				ForceNew:     false,
				ValidateFunc: validation.All(validation.IntAtLeast(0), validation.IntAtMost(128)),
			},
			"ttl": {
				Type:         schema.TypeInt,
				Description:  "How many seconds a value will be kept in the bucket",
				Optional:     true,
				ForceNew:     false,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
			},
			"max_value_size": {
				Type:         schema.TypeInt,
				Description:  "Maximum size of any value",
				Default:      -1,
				Optional:     true,
				ForceNew:     false,
				ValidateFunc: validation.IntAtLeast(-1),
			},
			"max_bucket_size": {
				Type:         schema.TypeInt,
				Description:  "Maximum size of the entire bucket",
				Default:      -1,
				Optional:     true,
				ForceNew:     false,
				ValidateFunc: validation.IntAtLeast(-1),
			},
			"placement_cluster": {
				Type:        schema.TypeString,
				Description: "Place the bucket in a specific cluster, influenced by placement_tags",
				Default:     "",
				Optional:    true,
			},
			"placement_tags": {
				Type:        schema.TypeList,
				Description: "Place the stream only on servers with these tags",
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"replicas": {
				Type:         schema.TypeInt,
				Description:  "Number of cluster replicas to store",
				Default:      1,
				Optional:     true,
				ForceNew:     false,
				ValidateFunc: validation.All(validation.IntAtLeast(1), validation.IntAtMost(5)),
			},
			"limit_marker_ttl": {
				Type:         schema.TypeInt,
				Description:  "Enables Per-Key TTLs and Limit Markers, duration specified in seconds",
				Optional:     true,
				ForceNew:     false,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
			},
		},
	}
}

func resourceKVBucketCreate(d *schema.ResourceData, m any) error {
	nc, err := getConnection(d, m)
	if err != nil {
		return err
	}
	defer nc.Close()

	name := d.Get("name").(string)
	history := d.Get("history").(int)
	ttl := d.Get("ttl").(int)
	maxV := d.Get("max_value_size").(int)
	maxB := d.Get("max_bucket_size").(int)
	replicas := d.Get("replicas").(int)
	descrption := d.Get("description").(string)
	limit_marker_ttl := d.Get("limit_marker_ttl").(int)

	var placement *jetstream.Placement
	c, ok := d.GetOk("placement_cluster")
	if ok {
		placement = &jetstream.Placement{Cluster: c.(string)}
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}

	known, err := js.KeyValue(ctx, name)
	if known != nil {
		return fmt.Errorf("bucket %s already exist", name)
	} else if err != nil {
		if !errors.Is(err, jetstream.ErrBucketNotFound) {
			return fmt.Errorf("failed to load KV bucket: %s", err)
		}
	}

	js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:         name,
		Description:    descrption,
		MaxValueSize:   int32(maxV),
		History:        uint8(history),
		TTL:            time.Duration(ttl) * time.Second,
		MaxBytes:       int64(maxB),
		Storage:        jetstream.FileStorage,
		Replicas:       replicas,
		Placement:      placement,
		LimitMarkerTTL: time.Duration(limit_marker_ttl) * time.Second,
	})

	d.SetId(fmt.Sprintf("JETSTREAM_KV_%s", name))

	return resourceKVBucketRead(d, m)
}

func resourceKVBucketRead(d *schema.ResourceData, m any) error {
	name, err := parseStreamKVID(d.Id())
	if err != nil {
		return err
	}

	nc, err := getConnection(d, m)
	if err != nil {
		return err
	}
	defer nc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}

	bucket, err := js.KeyValue(ctx, name)
	if err != nil {
		if errors.Is(err, jetstream.ErrBucketNotFound) {
			d.SetId("")
			return nil
		}
		return err
	}
	status, err := bucket.Status(ctx)
	if err != nil {
		return err
	}

	d.Set("name", status.Bucket())
	d.Set("history", status.History())
	d.Set("ttl", status.TTL().Seconds())

	jStatus := status.(*jetstream.KeyValueBucketStatus)
	si := jStatus.StreamInfo()

	d.Set("max_value_size", si.Config.MaxMsgSize)
	d.Set("max_bucket_size", si.Config.MaxBytes)
	d.Set("replicas", si.Config.Replicas)
	d.Set("description", si.Config.Description)

	if si.Config.Placement != nil {
		d.Set("placement_cluster", si.Config.Placement.Cluster)
		d.Set("placement_tags", si.Config.Placement.Tags)
	}

	d.Set("limit_marker_ttl", si.Config.SubjectDeleteMarkerTTL.Seconds())

	return nil
}

func resourceKVBucketUpdate(d *schema.ResourceData, m any) error {
	name := d.Get("name").(string)

	nc, err := getConnection(d, m)
	if err != nil {
		return err
	}
	defer nc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	bucket, err := js.KeyValue(ctx, name)
	if err != nil {
		return err
	}
	status, err := bucket.Status(ctx)
	if err != nil {
		return err
	}

	jStatus := status.(*jetstream.KeyValueBucketStatus)

	str, err := js.Stream(ctx, jStatus.StreamInfo().Config.Name)
	if err != nil {
		return err
	}

	cfg := jetstream.KeyValueConfig{
		Bucket:         bucket.Bucket(),
		Description:    str.CachedInfo().Config.Description,
		MaxValueSize:   str.CachedInfo().Config.MaxMsgSize,
		History:        uint8(status.History()),
		TTL:            status.TTL(),
		MaxBytes:       str.CachedInfo().Config.MaxBytes,
		Storage:        str.CachedInfo().Config.Storage,
		Replicas:       str.CachedInfo().Config.Replicas,
		Placement:      str.CachedInfo().Config.Placement,
		RePublish:      str.CachedInfo().Config.RePublish,
		Mirror:         str.CachedInfo().Config.Mirror,
		Sources:        str.CachedInfo().Config.Sources,
		Compression:    status.IsCompressed(),
		LimitMarkerTTL: status.LimitMarkerTTL(),
	}

	history := d.Get("history").(int)
	ttl := d.Get("ttl").(int)
	maxV := d.Get("max_value_size").(int)
	maxB := d.Get("max_bucket_size").(int)
	description := d.Get("description").(string)
	markerTTL := d.Get("limit_marker_ttl").(int)

	cfg.History = uint8(history)
	cfg.TTL = time.Duration(ttl) * time.Second
	cfg.MaxValueSize = int32(maxV)
	cfg.MaxBytes = int64(maxB)
	cfg.Description = description
	cfg.LimitMarkerTTL = time.Duration(markerTTL) * time.Second

	_, err = js.CreateOrUpdateKeyValue(ctx, cfg)
	if err != nil {
		return err
	}

	return resourceKVBucketRead(d, m)
}

func resourceKVBucketDelete(d *schema.ResourceData, m any) error {
	name := d.Get("name").(string)

	nc, err := getConnection(d, m)
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = js.DeleteKeyValue(ctx, name)
	if err == nats.ErrStreamNotFound {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}
