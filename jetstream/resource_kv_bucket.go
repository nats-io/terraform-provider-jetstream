package jetstream

import (
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/kv"
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
				Type:         schema.TypeString,
				Description:  "The name of the Bucket",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringMatch(regexp.MustCompile(kv.ValidBucketPattern), "invalid bucket name"),
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
			"placement": {
				Type:        schema.TypeString,
				Description: "The cluster to place the bucket in",
				Optional:    true,
				ForceNew:    true,
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
				Default:      0,
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
	placement := d.Get("placement").(string)
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

	_, err = kv.NewBucket(nc, name, kv.WithHistory(uint64(history)),
		kv.WithReplicas(uint(replicas)),
		kv.WithMaxValueSize(int32(maxV)),
		kv.WithMaxBucketSize(int64(maxB)),
		kv.WithTTL(time.Duration(ttl)*time.Second),
		kv.WithPlacementCluster(placement),
	)
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

	bucket, err := kv.NewClient(nc, name)
	if err != nil {
		return err
	}

	status, err := bucket.Status()
	if err != nil {
		return err
	}

	ok, failed := status.Replicas()

	d.Set("name", status.Bucket())
	d.Set("history", status.History())
	d.Set("ttl", status.TTL().Seconds())

	if status.BucketLocation() == "unknown" {
		d.Set("placement", "")
	} else {
		d.Set("placement", status.BucketLocation())
	}

	d.Set("max_value_size", status.MaxValueSize())
	d.Set("max_bucket_size", status.MaxBucketSize())
	d.Set("replicas", ok+failed)

	return nil
}

func resourceKVBucketUpdate(d *schema.ResourceData, m interface{}) error {
	name := d.Get("name").(string)

	nc, mgr, err := m.(func() (*nats.Conn, *jsm.Manager, error))()
	if err != nil {
		return err
	}
	defer nc.Close()

	bucket, err := kv.NewClient(nc, name)
	if err != nil {
		return err
	}

	status, err := bucket.Status()
	if err != nil {
		return err
	}

	str, err := mgr.LoadStream(status.BackingStore())
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

	bucket, err := kv.NewBucket(nc, name)
	if err != nil {
		return err
	}

	return bucket.Destroy()
}
