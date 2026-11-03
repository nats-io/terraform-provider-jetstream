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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func resourceObjBucket() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceObjBucketCreate,
		ReadContext:   resourceObjBucketRead,
		UpdateContext: resourceObjBucketUpdate,
		DeleteContext: resourceObjBucketDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Description: "The name of the Object Store bucket",
				Required:    true,
				ForceNew:    true,
			},
			"description": {
				Type:        schema.TypeString,
				Description: "Contains additional information about this bucket",
				Optional:    true,
				ForceNew:    false,
			},
			"storage": {
				Type:             schema.TypeString,
				Description:      "The storage engine to use to back the bucket",
				Default:          "file",
				ForceNew:         true,
				Optional:         true,
				ValidateDiagFunc: validateStorageTypeString(),
			},
			"ttl": {
				Type:         schema.TypeInt,
				Description:  "How many seconds an object will be kept in the bucket",
				Optional:     true,
				ForceNew:     false,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
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
			"compression": {
				Type:        schema.TypeBool,
				Description: "Enables compression for objects stored in the bucket",
				Optional:    true,
				ForceNew:    false,
				Default:     false,
			},
		},
	}
}

func resourceObjBucketCreate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nc, err := getConnection(d, m)
	if err != nil {
		return diag.FromErr(err)
	}
	defer nc.Close()

	name := d.Get("name").(string)
	ttl := d.Get("ttl").(int)
	maxB := d.Get("max_bucket_size").(int)
	replicas := d.Get("replicas").(int)
	description := d.Get("description").(string)
	compression := d.Get("compression").(bool)

	var storage jetstream.StorageType
	switch d.Get("storage").(string) {
	case "file":
		storage = jetstream.FileStorage
	case "memory":
		storage = jetstream.MemoryStorage
	}

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

	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	js, err := jetstream.New(nc)
	if err != nil {
		return diag.FromErr(err)
	}

	known, err := js.ObjectStore(opCtx, name)
	if known != nil {
		return diag.Errorf("bucket %s already exist", name)
	} else if err != nil {
		if !errors.Is(err, jetstream.ErrBucketNotFound) {
			return diag.Errorf("failed to load object store bucket: %s", err)
		}
	}

	_, err = js.CreateObjectStore(opCtx, jetstream.ObjectStoreConfig{
		Bucket:      name,
		Description: description,
		TTL:         time.Duration(ttl) * time.Second,
		MaxBytes:    int64(maxB),
		Storage:     storage,
		Replicas:    replicas,
		Placement:   placement,
		Compression: compression,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("JETSTREAM_OBJ_%s", name))

	return resourceObjBucketRead(ctx, d, m)
}

func resourceObjBucketRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	name, err := parseStreamObjID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	nc, err := getConnection(d, m)
	if err != nil {
		return diag.FromErr(err)
	}
	defer nc.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	js, err := jetstream.New(nc)
	if err != nil {
		return diag.FromErr(err)
	}

	bucket, err := js.ObjectStore(ctx, name)
	if err != nil {
		if errors.Is(err, jetstream.ErrBucketNotFound) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	status, err := bucket.Status(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("name", status.Bucket())
	d.Set("description", status.Description())
	d.Set("ttl", status.TTL().Seconds())
	d.Set("replicas", status.Replicas())
	d.Set("compression", status.IsCompressed())

	switch status.Storage() {
	case jetstream.FileStorage:
		d.Set("storage", "file")
	case jetstream.MemoryStorage:
		d.Set("storage", "memory")
	}

	oStatus := status.(*jetstream.ObjectBucketStatus)
	si := oStatus.StreamInfo()

	d.Set("max_bucket_size", si.Config.MaxBytes)

	if si.Config.Placement != nil {
		d.Set("placement_cluster", si.Config.Placement.Cluster)
		d.Set("placement_tags", si.Config.Placement.Tags)
	}

	return nil
}

func resourceObjBucketUpdate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	name := d.Get("name").(string)

	nc, err := getConnection(d, m)
	if err != nil {
		return diag.FromErr(err)
	}
	defer nc.Close()

	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	js, err := jetstream.New(nc)
	if err != nil {
		return diag.FromErr(err)
	}
	bucket, err := js.ObjectStore(opCtx, name)
	if err != nil {
		return diag.FromErr(err)
	}
	status, err := bucket.Status(opCtx)
	if err != nil {
		return diag.FromErr(err)
	}

	oStatus := status.(*jetstream.ObjectBucketStatus)

	str, err := js.Stream(opCtx, oStatus.StreamInfo().Config.Name)
	if err != nil {
		return diag.FromErr(err)
	}

	cfg := jetstream.ObjectStoreConfig{
		Bucket:      name,
		Description: str.CachedInfo().Config.Description,
		TTL:         status.TTL(),
		MaxBytes:    str.CachedInfo().Config.MaxBytes,
		Storage:     str.CachedInfo().Config.Storage,
		Replicas:    str.CachedInfo().Config.Replicas,
		Compression: status.IsCompressed(),
		Metadata:    str.CachedInfo().Config.Metadata,
	}

	ttl := d.Get("ttl").(int)
	maxB := d.Get("max_bucket_size").(int)
	replicas := d.Get("replicas").(int)
	description := d.Get("description").(string)
	compression := d.Get("compression").(bool)

	var placement *jetstream.Placement
	if cluster, ok := d.GetOk("placement_cluster"); ok {
		placement = &jetstream.Placement{Cluster: cluster.(string)}
		if rawTags, ok := d.GetOk("placement_tags"); ok {
			tagList := rawTags.([]any)
			tags := make([]string, len(tagList))
			for i, tag := range tagList {
				tags[i] = tag.(string)
			}
			placement.Tags = tags
		}
	}

	cfg.TTL = time.Duration(ttl) * time.Second
	cfg.MaxBytes = int64(maxB)
	cfg.Replicas = replicas
	cfg.Description = description
	cfg.Placement = placement
	cfg.Compression = compression

	_, err = js.CreateOrUpdateObjectStore(opCtx, cfg)
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceObjBucketRead(ctx, d, m)
}

func resourceObjBucketDelete(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	name := d.Get("name").(string)

	nc, err := getConnection(d, m)
	if err != nil {
		return diag.FromErr(err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		return diag.FromErr(err)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err = js.DeleteObjectStore(ctx, name)
	if errors.Is(err, jetstream.ErrBucketNotFound) || errors.Is(err, nats.ErrStreamNotFound) {
		return nil
	} else if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
