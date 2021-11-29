package jetstream

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

func resourceKVBucket() *schema.Resource {
	return &schema.Resource{
		Create: resourceKVBucketCreate,
		Read:   resourceKVBucketRead,
		Update: resourceKVBucketUpdate,
		Delete: resourceKVBucketDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Description: "The name of the Bucket",
				Required:    true,
				ForceNew:    true,
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
			"replicas": {
				Type:         schema.TypeInt,
				Description:  "Number of cluster replicas to store",
				Default:      1,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validation.All(validation.IntAtLeast(1), validation.IntAtMost(5)),
			},
		},
	}
}

func resourceKVBucketCreate(d *schema.ResourceData, m interface{}) error {
	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
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

	known, err := mgr.IsKnownStream("KV_" + name)
	if err != nil {
		return err
	}
	if known {
		return fmt.Errorf("bucket %s already exist", name)
	}

	js, err := nc.JetStream()
	if err != nil {
		return err
	}

	_, err = js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:       name,
		MaxValueSize: int32(maxV),
		History:      uint8(history),
		TTL:          time.Duration(ttl) * time.Second,
		MaxBytes:     int64(maxB),
		Storage:      nats.FileStorage,
		Replicas:     replicas,
	})
	if err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("JETSTREAM_KV_%s", name))

	return resourceKVBucketRead(d, m)
}

func resourceKVBucketRead(d *schema.ResourceData, m interface{}) error {
	name, err := parseStreamKVID(d.Id())
	if err != nil {
		return err
	}

	nc, _, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	bucket, err := js.KeyValue(name)
	if err != nil {
		return err
	}
	status, err := bucket.Status()
	if err != nil {
		return err
	}

	d.Set("name", status.Bucket())
	d.Set("history", status.History())
	d.Set("ttl", status.TTL().Seconds())

	jStatus := status.(*nats.KeyValueBucketStatus)
	d.Set("max_value_size", jStatus.StreamInfo().Config.MaxMsgSize)
	d.Set("max_bucket_size", jStatus.StreamInfo().Config.MaxBytes)
	d.Set("replicas", jStatus.StreamInfo().Config.Replicas)

	return nil
}

func resourceKVBucketUpdate(d *schema.ResourceData, m interface{}) error {
	name := d.Get("name").(string)

	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	bucket, err := js.KeyValue(name)
	if err != nil {
		return err
	}
	status, err := bucket.Status()
	if err != nil {
		return err
	}
	jStatus := status.(*nats.KeyValueBucketStatus)

	str, err := mgr.LoadStream(jStatus.StreamInfo().Config.Name)
	if err != nil {
		return err
	}

	history := d.Get("history").(int)
	ttl := d.Get("ttl").(int)
	maxV := d.Get("max_value_size").(int)
	maxB := d.Get("max_bucket_size").(int)

	cfg := str.Configuration()
	cfg.MaxAge = time.Duration(ttl) * time.Second
	cfg.MaxMsgSize = int32(maxV)
	cfg.MaxBytes = int64(maxB)
	cfg.MaxMsgsPer = int64(history)

	err = str.UpdateConfiguration(cfg)
	if err != nil {
		return err
	}

	return resourceKVBucketRead(d, m)
}

func resourceKVBucketDelete(d *schema.ResourceData, m interface{}) error {
	name := d.Get("name").(string)

	nc, _, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	err = js.DeleteKeyValue(name)
	if err == nats.ErrStreamNotFound {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}
