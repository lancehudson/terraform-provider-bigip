package bigip

import (
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/scottdware/go-bigip"
	"regexp"
	"strings"
)

func resourceBigipLtmNode() *schema.Resource {
	return &schema.Resource{
		Create: resourceBigipLtmNodeCreate,
		Read:   resourceBigipLtmNodeRead,
		//Update: resourceBigipLtmNodeUpdate,
		Delete: resourceBigipLtmNodeDelete,
		Exists: resourceBigipLtmNodeExists,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the node",
				ForceNew:    true,
			},

			"partition": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     DEFAULT_PARTITION,
				Description: "LTM Partition",
				ForceNew:    true,
			},

			"address": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Address of the node",
				ForceNew:    true,
			},
		},
	}
}

func resourceBigipLtmNodeCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*bigip.BigIP)

	address := d.Get("address").(string)
	partition := d.Get("partition").(string)
	var name string
	if v, ok := d.GetOk("name"); ok {
		name = v.(string)
	} else {
		name = address
	}

	log.Println("[INFO] Creating node " + name + "::" + address)
	err := client.CreateNode(
		name,
		partition,
		address,
	)
	if err != nil {
		return err
	}

	d.SetId(name)

	return resourceBigipLtmNodeRead(d, meta)
}

func resourceBigipLtmNodeRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*bigip.BigIP)

	name := d.Id()
	partition := d.Get("partition").(string)

	log.Println("[INFO] Fetching node " + name)

	node, err := client.GetNode(name, partition)
	if err != nil {
		return err
	}

	partition = node.Partition
	if partition == "" {
		partition = DEFAULT_PARTITION
	}

	d.Set("name", node.Name)
	d.Set("partition", partition)
	d.Set("address", node.Address)

	return nil
}

func resourceBigipLtmNodeExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(*bigip.BigIP)

	name := d.Id()
	partition := d.Get("partition").(string)
	log.Println("[INFO] Fetching node " + name)

	vs, err := client.GetNode(name, partition)
	if err != nil {
		return false, err
	}

	if vs == nil {
		d.SetId("")
	}

	return vs != nil, nil
}

func resourceBigipLtmNodeUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*bigip.BigIP)

	name := d.Id()
	partition := d.Get("partition").(string)

	vs := &bigip.Node{
		Name:    name,
		Address: d.Get("address").(string),
		Partition: partition,
	}

	return client.ModifyNode(name, partition, vs)
}

func resourceBigipLtmNodeDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*bigip.BigIP)

	name := d.Id()
	partition := d.Get("partition").(string)
	log.Println("[INFO] Deleting node " + name)

	err := client.DeleteNode(name, partition)
	regex := regexp.MustCompile("referenced by a member of pool '\\/\\w+/([\\w-_.]+)")
	for err != nil {
		log.Println("[INFO] Deleting %s from pools...", name)
		parts := regex.FindStringSubmatch(err.Error())
		if len(parts) > 1 {
			poolName := parts[1]
			members, e := client.PoolMembers(poolName, partition)
			if e != nil {
				return e
			}
			for _, member := range members {
				if strings.HasPrefix(member, name+":") {
					e = client.DeletePoolMember(poolName, partition, member)
					if e != nil {
						return e
					}
				}
			}
			err = client.DeleteNode(name, partition)
		} else {
			break
		}
	}
	return err
}
