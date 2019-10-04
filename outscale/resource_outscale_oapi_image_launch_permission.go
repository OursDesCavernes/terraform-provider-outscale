package outscale

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/terraform-providers/terraform-provider-outscale/osc/oapi"
	"github.com/terraform-providers/terraform-provider-outscale/utils"
)

func resourceOutscaleOAPIImageLaunchPermission() *schema.Resource {
	return &schema.Resource{
		Exists: resourceOutscaleOAPIImageLaunchPermissionExists,
		Create: resourceOutscaleOAPIImageLaunchPermissionCreate,
		Read:   resourceOutscaleOAPIImageLaunchPermissionRead,
		Delete: resourceOutscaleOAPIImageLaunchPermissionDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"image_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"permission_additions": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"global_permission": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
							Default:  "false",
						},
						"account_ids": &schema.Schema{
							Type:     schema.TypeList,
							Optional: true,
							ForceNew: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"description": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"permissions_to_launch": &schema.Schema{
				Type:     schema.TypeList,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"global_permission": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"account_ids": &schema.Schema{
							Type:     schema.TypeList,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"request_id": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceOutscaleOAPIImageLaunchPermissionExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	conn := meta.(*OutscaleClient).OAPI

	imageID := d.Get("image_id").(string)
	return hasOAPILaunchPermission(conn, imageID)
}

func resourceOutscaleOAPIImageLaunchPermissionCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*OutscaleClient).OAPI

	imageID, iok := d.GetOk("image_id")

	fmt.Println("Creating Outscale Image Launch Permission, image_id", imageID.(string))

	if !iok {
		return fmt.Errorf("please provide the required attribute image_id")
	}

	request := &oapi.UpdateImageRequest{
		ImageId: imageID.(string),
	}

	//request.Attribute = aws.String("launchPermission")
	launchPermission := oapi.PermissionsOnResourceCreation{}

	if v, ok := d.GetOk("permission_additions"); ok {
		add := v.([]interface{})

		if len(add) > 0 {
			accountIds := make([]string, len(add))
			var globalPermission bool

			att := add[0].(map[string]interface{})
			if g, ok := att["global_permission"]; ok {
				globalPermission, _ = strconv.ParseBool(g.(string))
			}
			if g, ok := att["account_ids"]; ok {
				accountIds = make([]string, len(g.([]interface{})))

				for k, v := range g.([]interface{}) {
					accountIds[k] = v.(string)
				}
			}

			launchPermission.Additions = oapi.PermissionsOnResource{
				AccountIds:       accountIds,
				GlobalPermission: globalPermission,
			}
		}
		request.PermissionsToLaunch = launchPermission
	}

	var resp *oapi.POST_UpdateImageResponses
	err := resource.Retry(5*time.Minute, func() *resource.RetryError {
		var err error
		resp, err = conn.POST_UpdateImage(*request)
		if err != nil {
			if strings.Contains(err.Error(), "RequestLimitExceeded:") {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})

	var errString string

	if err != nil || resp.OK == nil {
		if err != nil {
			errString = err.Error()
		} else if resp.Code401 != nil {
			errString = fmt.Sprintf("Status Code: 401, %s", utils.ToJSONString(resp.Code401))
		} else if resp.Code400 != nil {
			errString = fmt.Sprintf("Status Code: 400, %s", utils.ToJSONString(resp.Code400))
		} else if resp.Code500 != nil {
			errString = fmt.Sprintf("Status Code: 500, %s", utils.ToJSONString(resp.Code500))
		}

		return fmt.Errorf("error creating omi launch permission: %s", errString)
	}

	d.SetId(imageID.(string))

	return resourceOutscaleOAPIImageLaunchPermissionRead(d, meta)
}

func resourceOutscaleOAPIImageLaunchPermissionRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*OutscaleClient).OAPI

	var attrs *oapi.POST_ReadImagesResponses
	var err error
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		attrs, err = conn.POST_ReadImages(oapi.ReadImagesRequest{
			Filters: oapi.FiltersImage{
				ImageIds: []string{d.Id()},
			},
		})
		if err != nil {
			if strings.Contains(err.Error(), "RequestLimitExceeded:") {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})

	var errString string

	if err != nil || attrs.OK == nil {
		if err != nil {
			// When an AMI disappears out from under a launch permission resource, we will
			// see either InvalidAMIID.NotFound or InvalidAMIID.Unavailable.
			if strings.Contains(fmt.Sprint(err), "InvalidAMIID") {
				log.Printf("[DEBUG] %s no longer exists, so we'll drop launch permission for the state", d.Id())
				return nil
			}
			errString = err.Error()
		} else if attrs.Code401 != nil {
			errString = fmt.Sprintf("ErrorCode: 401, %s", utils.ToJSONString(attrs.Code401))
		} else if attrs.Code400 != nil {
			errString = fmt.Sprintf("ErrorCode: 400, %s", utils.ToJSONString(attrs.Code400))
		} else if attrs.Code500 != nil {
			errString = fmt.Sprintf("ErrorCode: 500, %s", utils.ToJSONString(attrs.Code500))
		}

		return fmt.Errorf("Error reading Outscale image permission: %s", errString)
	}

	result := attrs.OK.Images[0]

	d.Set("request_id", attrs.OK.ResponseContext.RequestId)
	d.Set("description", result.Description)

	lp := make(map[string]interface{})
	lp["global_permission"] = strconv.FormatBool(result.PermissionsToLaunch.GlobalPermission)
	lp["account_ids"] = result.PermissionsToLaunch.AccountIds

	return d.Set("permissions_to_launch", []map[string]interface{}{lp})
}

func resourceOutscaleOAPIImageLaunchPermissionDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*OutscaleClient).OAPI

	imageID, iok := d.GetOk("image_id")
	permission, lok := d.GetOk("permission_additions")

	if !iok {
		return fmt.Errorf("please provide the required attribute image_id")
	}

	request := &oapi.UpdateImageRequest{
		ImageId: imageID.(string),
	}

	if lok {
		//request.Attribute = aws.String("launchPermission")
		launchPermission := oapi.PermissionsOnResourceCreation{}

		delete := permission.([]interface{})

		if len(delete) > 0 {
			accountIds := make([]string, 0)
			var globalPermission bool

			att := delete[0].(map[string]interface{})
			if g, ok := att["global_permission"]; ok {
				globalPermission, _ = strconv.ParseBool(g.(string))
			}
			if g, ok := att["account_ids"]; ok {
				accountIds = make([]string, len(g.([]interface{})))

				for k, v := range g.([]interface{}) {
					accountIds[k] = v.(string)
				}
			}

			launchPermission.Removals = oapi.PermissionsOnResource{
				AccountIds:       accountIds,
				GlobalPermission: globalPermission,
			}
		}

		request.PermissionsToLaunch = launchPermission
	}

	var resp *oapi.POST_UpdateImageResponses
	err := resource.Retry(5*time.Minute, func() *resource.RetryError {
		var err error
		resp, err = conn.POST_UpdateImage(*request)
		if err != nil {
			if strings.Contains(err.Error(), "RequestLimitExceeded:") {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})

	var errString string

	if err != nil || resp.OK == nil {
		if err != nil {
			errString = err.Error()
		} else if resp.Code401 != nil {
			errString = fmt.Sprintf("Status Code: 401, %s", utils.ToJSONString(resp.Code401))
		} else if resp.Code400 != nil {
			errString = fmt.Sprintf("Status Code: 400, %s", utils.ToJSONString(resp.Code400))
		} else if resp.Code500 != nil {
			errString = fmt.Sprintf("Status Code: 500, %s", utils.ToJSONString(resp.Code500))
		}

		return fmt.Errorf("error removing omi launch permission: %s", errString)
	}

	d.SetId("")

	return nil
}

func hasOAPILaunchPermission(conn *oapi.Client, imageID string) (bool, error) {
	var attrs *oapi.POST_ReadImagesResponses
	var err error
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		attrs, err = conn.POST_ReadImages(oapi.ReadImagesRequest{
			Filters: oapi.FiltersImage{
				ImageIds: []string{imageID},
			},
		})
		if err != nil {
			if strings.Contains(err.Error(), "RequestLimitExceeded:") {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})

	var errString string

	if err != nil || attrs.OK == nil {
		if err != nil {
			// When an AMI disappears out from under a launch permission resource, we will
			// see either InvalidAMIID.NotFound or InvalidAMIID.Unavailable.
			if strings.Contains(fmt.Sprint(err), "InvalidAMIID") {
				log.Printf("[DEBUG] %s no longer exists, so we'll drop launch permission for the state", imageID)
				return false, nil
			}
			errString = err.Error()
		} else if attrs.Code401 != nil {
			errString = fmt.Sprintf("ErrorCode: 401, %s", utils.ToJSONString(attrs.Code401))
		} else if attrs.Code400 != nil {
			errString = fmt.Sprintf("ErrorCode: 400, %s", utils.ToJSONString(attrs.Code400))
		} else if attrs.Code500 != nil {
			errString = fmt.Sprintf("ErrorCode: 500, %s", utils.ToJSONString(attrs.Code500))
		}

		return false, fmt.Errorf("Error creating Outscale VM volume: %s", errString)
	}

	if len(attrs.OK.Images) == 0 {
		log.Printf("[DEBUG] %s no longer exists, so we'll drop launch permission for the state", imageID)
		return false, nil
	}

	result := attrs.OK.Images[0]
	fmt.Printf("RESULT: %+v\n", result)

	if len(result.PermissionsToLaunch.AccountIds) > 0 {
		return true, nil
	}
	return false, nil
}