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
	"github.com/nats-io/nats.go/jetstream"
)

func resourceKVEntry() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKVEntryCreate,
		ReadContext:   resourceKVEntryRead,
		UpdateContext: resourceKVEntryUpdate,
		DeleteContext: resourceKVEntryDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"bucket": {
				Type:        schema.TypeString,
				Description: "The name of the bucket",
				Required:    true,
				ForceNew:    true,
			},
			"key": {
				Type:        schema.TypeString,
				Description: "The key of the entry",
				Required:    true,
				ForceNew:    true,
			},
			"value": {
				Type:        schema.TypeString,
				Description: "The value of the entry",
				Required:    true,
				ForceNew:    false,
			},
			"revision": {
				Type:        schema.TypeInt,
				Description: "The revision of the entry",
				Computed:    true,
			},
		},
	}
}

func resourceKVEntryCreate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nc, err := getConnection(d, m)
	if err != nil {
		return diag.FromErr(err)
	}
	defer nc.Close()

	bucket := d.Get("bucket").(string)
	key := d.Get("key").(string)
	value := d.Get("value").(string)

	js, err := jetstream.New(nc)
	if err != nil {
		return diag.FromErr(err)
	}
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	kv, err := js.KeyValue(opCtx, bucket)
	if err != nil {
		return diag.FromErr(err)
	}
	_, err = kv.Put(opCtx, key, []byte(value))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("JETSTREAM_KV_%s_ENTRY_%s", bucket, key))

	return resourceKVEntryRead(ctx, d, m)
}

func resourceKVEntryRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	bucket, key, err := parseStreamKVEntryID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

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

	kv, err := js.KeyValue(ctx, bucket)
	if err != nil {
		if errors.Is(err, jetstream.ErrBucketNotFound) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	entry, err := kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	d.Set("bucket", entry.Bucket())
	d.Set("key", entry.Key())
	d.Set("value", string(entry.Value()))
	d.Set("revision", entry.Revision())

	return nil
}

func resourceKVEntryUpdate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	bucket := d.Get("bucket").(string)

	nc, err := getConnection(d, m)
	if err != nil {
		return diag.FromErr(err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		return diag.FromErr(err)
	}
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	kv, err := js.KeyValue(opCtx, bucket)
	if err != nil {
		return diag.FromErr(err)
	}

	key := d.Get("key").(string)
	value := d.Get("value").(string)

	_, err = kv.Put(opCtx, key, []byte(value))
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceKVEntryRead(ctx, d, m)
}

func resourceKVEntryDelete(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	bucket := d.Get("bucket").(string)

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

	kv, err := js.KeyValue(ctx, bucket)
	if err != nil {
		return diag.FromErr(err)
	}
	err = kv.Delete(ctx, d.Get("key").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
