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
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/nats-io/nats.go"
)

func resourceKVEntry() *schema.Resource {
	return &schema.Resource{
		Create: resourceKVEntryCreate,
		Read:   resourceKVEntryRead,
		Update: resourceKVEntryUpdate,
		Delete: resourceKVEntryDelete,
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

func resourceKVEntryCreate(d *schema.ResourceData, m any) error {
	nc, err := getConnection(d, m)
	if err != nil {
		return err
	}
	defer nc.Close()

	bucket := d.Get("bucket").(string)
	key := d.Get("key").(string)
	value := d.Get("value").(string)

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	kv, err := js.KeyValue(bucket)
	if err != nil {
		return err
	}
	_, err = kv.Put(key, []byte(value))
	if err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("JETSTREAM_KV_%s_ENTRY_%s", bucket, key))

	return resourceKVEntryRead(d, m)
}

func resourceKVEntryRead(d *schema.ResourceData, m any) error {
	bucket, key, err := parseStreamKVEntryID(d.Id())
	if err != nil {
		return err
	}

	nc, err := getConnection(d, m)
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	kv, err := js.KeyValue(bucket)
	if err != nil {
		if errors.Is(err, nats.ErrBucketNotFound) {
			d.SetId("")
			return nil
		}
		return err
	}
	entry, err := kv.Get(key)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			d.SetId("")
			return nil
		}
		return err
	}

	d.Set("bucket", entry.Bucket())
	d.Set("key", entry.Key())
	d.Set("value", string(entry.Value()))
	d.Set("revision", entry.Revision())

	return nil
}

func resourceKVEntryUpdate(d *schema.ResourceData, m any) error {
	bucket := d.Get("bucket").(string)

	nc, err := getConnection(d, m)
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	kv, err := js.KeyValue(bucket)
	if err != nil {
		return err
	}

	key := d.Get("key").(string)
	value := d.Get("value").(string)

	_, err = kv.Put(key, []byte(value))
	if err != nil {
		return err
	}

	return resourceKVEntryRead(d, m)
}

func resourceKVEntryDelete(d *schema.ResourceData, m any) error {
	bucket := d.Get("bucket").(string)

	nc, err := getConnection(d, m)
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	kv, err := js.KeyValue(bucket)
	if err != nil {
		return err
	}
	err = kv.Delete(d.Get("key").(string))
	if err != nil {
		return err
	}

	return nil
}
