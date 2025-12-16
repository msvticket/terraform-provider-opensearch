package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/olivere/elastic/uritemplates"

	elastic7 "github.com/olivere/elastic/v7"
)

var openSearchWorkspaceSchema = map[string]*schema.Schema{
	"workspace_id": {
		Type:        schema.TypeString,
		Computed:    true,
		Description: "The ID of the workspace.",
	},
	"name": {
		Type:        schema.TypeString,
		Required:    true,
		Description: "The name of the workspace.",
	},
	"description": {
		Type:        schema.TypeString,
		Optional:    true,
		Description: "Description of the workspace.",
	},
	"features": {
		Type:        schema.TypeSet,
		Optional:    true,
		Elem:        &schema.Schema{Type: schema.TypeString},
		Description: "List of features enabled for this workspace.",
	},
	"permissions": {
		Type:        schema.TypeList,
		Optional:    true,
		MaxItems:    1,
		Description: "Permissions configuration for the workspace.",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"library_write": {
					Type:        schema.TypeList,
					Optional:    true,
					MaxItems:    1,
					Description: "Users and groups with write permissions.",
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"users": {
								Type:        schema.TypeSet,
								Optional:    true,
								Description: "List of users with write permissions.",
								Elem:        &schema.Schema{Type: schema.TypeString},
							},
							"groups": {
								Type:        schema.TypeSet,
								Optional:    true,
								Description: "List of groups with write permissions.",
								Elem:        &schema.Schema{Type: schema.TypeString},
							},
						},
					},
				},
				"library_read": {
					Type:        schema.TypeList,
					Optional:    true,
					MaxItems:    1,
					Description: "Users and groups with read permissions.",
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"users": {
								Type:        schema.TypeSet,
								Optional:    true,
								Description: "List of users with read permissions.",
								Elem:        &schema.Schema{Type: schema.TypeString},
							},
							"groups": {
								Type:        schema.TypeSet,
								Optional:    true,
								Description: "List of groups with read permissions.",
								Elem:        &schema.Schema{Type: schema.TypeString},
							},
						},
					},
				},
			},
		},
	},
}

func resourceOpenSearchWorkspace() *schema.Resource {
	return &schema.Resource{
		Create: resourceOpensearchWorkspaceCreate,
		Read:   resourceOpensearchWorkspaceRead,
		Update: resourceOpensearchWorkspaceUpdate,
		Delete: resourceOpensearchWorkspaceDelete,
		Schema: openSearchWorkspaceSchema,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Description: "Provides an OpenSearch workspace resource. Workspaces allow you to segregate your OpenSearch Dashboards into multiple tenants. Please refer to the OpenSearch Dashboards workspace documentation for details.",
	}
}

func resourceOpensearchWorkspaceCreate(d *schema.ResourceData, m interface{}) error {
	if _, err := resourceOpensearchPutWorkspace(d, m); err != nil {
		log.Printf("[INFO] Failed to create workspace: %+v", err)
		return err
	}

	// The API returns the workspace ID in the response, we'll read it back
	return resourceOpensearchWorkspaceRead(d, m)
}

func resourceOpensearchWorkspaceRead(d *schema.ResourceData, m interface{}) error {
	workspaceID := d.Id()
	res, err := resourceOpensearchGetWorkspace(workspaceID, m)

	if err != nil {
		if elastic7.IsNotFound(err) {
			log.Printf("[WARN] Workspace (%s) not found, removing from state", workspaceID)
			d.SetId("")
			return nil
		}
		return err
	}

	ds := &resourceDataSetter{d: d}
	ds.set("workspace_id", res.ID)
	ds.set("name", res.Name)
	ds.set("description", res.Description)
	if len(res.Features) > 0 {
		ds.set("features", flattenStringSet(res.Features))
	}
	if res.Permissions != nil {
		ds.set("permissions", flattenWorkspacePermissions(res.Permissions))
	}

	return ds.err
}

func resourceOpensearchWorkspaceUpdate(d *schema.ResourceData, m interface{}) error {
	if _, err := resourceOpensearchPutWorkspace(d, m); err != nil {
		return err
	}

	return resourceOpensearchWorkspaceRead(d, m)
}

func resourceOpensearchWorkspaceDelete(d *schema.ResourceData, m interface{}) error {
	workspaceID := d.Id()

	path, err := uritemplates.Expand("/api/workspaces/{id}", map[string]string{
		"id": workspaceID,
	})
	if err != nil {
		return fmt.Errorf("error building URL path for workspace: %+v", err)
	}

	osClient, err := getClient(m.(*ProviderConf))
	if err != nil {
		return err
	}

	_, err = osClient.PerformRequest(context.TODO(), elastic7.PerformRequestOptions{
		Method:           "DELETE",
		Path:             path,
		RetryStatusCodes: []int{http.StatusConflict, http.StatusInternalServerError},
		Retrier: elastic7.NewBackoffRetrier(
			elastic7.NewExponentialBackoff(100*time.Millisecond, 30*time.Second),
		),
	})

	return err
}

func resourceOpensearchGetWorkspace(workspaceID string, m interface{}) (*WorkspaceBody, error) {
	var err error
	workspace := new(WorkspaceBody)

	path, err := uritemplates.Expand("/api/workspaces/{id}", map[string]string{
		"id": workspaceID,
	})

	if err != nil {
		return workspace, fmt.Errorf("error building URL path for workspace: %+v", err)
	}

	var body json.RawMessage
	osClient, err := getClient(m.(*ProviderConf))
	if err != nil {
		return workspace, err
	}

	var res *elastic7.Response
	res, err = osClient.PerformRequest(context.TODO(), elastic7.PerformRequestOptions{
		Method: "GET",
		Path:   path,
	})
	if err != nil {
		return workspace, err
	}
	body = res.Body

	var workspaceResponse WorkspaceResponse
	if err := json.Unmarshal(body, &workspaceResponse); err != nil {
		return workspace, fmt.Errorf("error unmarshalling workspace body: %+v: %+v", err, body)
	}

	if !workspaceResponse.Success {
		return workspace, fmt.Errorf("workspace get request failed")
	}

	return &workspaceResponse.Result, nil
}

func resourceOpensearchPutWorkspace(d *schema.ResourceData, m interface{}) (*WorkspaceResponse, error) {
	response := new(WorkspaceResponse)

	workspaceDefinition := WorkspaceBody{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
	}

	if v, ok := d.GetOk("features"); ok {
		workspaceDefinition.Features = expandStringList(v.(*schema.Set).List())
	}

	if v, ok := d.GetOk("permissions"); ok {
		workspaceDefinition.Permissions = expandWorkspacePermissions(v.([]interface{}))
	}

	workspaceJSON, err := json.Marshal(workspaceDefinition)
	if err != nil {
		return response, fmt.Errorf("Body Error : %s", workspaceJSON)
	}

	var path string
	var method string

	// If we have an ID, this is an update
	if d.Id() != "" {
		path, err = uritemplates.Expand("/api/workspaces/{id}", map[string]string{
			"id": d.Id(),
		})
		if err != nil {
			return response, fmt.Errorf("error building URL path for workspace: %+v", err)
		}
		method = "PUT"
	} else {
		// This is a create
		path = "/api/workspaces"
		method = "POST"
	}

	var body json.RawMessage
	osClient, err := getClient(m.(*ProviderConf))
	if err != nil {
		return nil, err
	}

	var res *elastic7.Response
	res, err = osClient.PerformRequest(context.TODO(), elastic7.PerformRequestOptions{
		Method:           method,
		Path:             path,
		Body:             string(workspaceJSON),
		RetryStatusCodes: []int{http.StatusConflict, http.StatusInternalServerError},
		Retrier: elastic7.NewBackoffRetrier(
			elastic7.NewExponentialBackoff(100*time.Millisecond, 30*time.Second),
		),
	})
	if err != nil {
		return response, err
	}
	body = res.Body

	if err := json.Unmarshal(body, response); err != nil {
		return response, fmt.Errorf("error unmarshalling workspace body: %+v: %+v", err, body)
	}

	if !response.Success {
		return response, fmt.Errorf("workspace operation failed")
	}

	// Set the ID from the response
	d.SetId(response.Result.ID)

	return response, nil
}

// WorkspaceBody represents the structure of a workspace
type WorkspaceBody struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Features    []string               `json:"features,omitempty"`
	Permissions *WorkspacePermissions  `json:"permissions,omitempty"`
}

// WorkspaceResponse represents the API response for workspace operations
type WorkspaceResponse struct {
	Success bool          `json:"success"`
	Result  WorkspaceBody `json:"result"`
}

// WorkspacePermissions represents the permissions structure for a workspace
type WorkspacePermissions struct {
	LibraryWrite *PermissionLevel `json:"library_write,omitempty"`
	LibraryRead  *PermissionLevel `json:"library_read,omitempty"`
}

// PermissionLevel represents users and groups at a permission level
type PermissionLevel struct {
	Users  []string `json:"users,omitempty"`
	Groups []string `json:"groups,omitempty"`
}

// expandWorkspacePermissions converts Terraform schema data to WorkspacePermissions
func expandWorkspacePermissions(perms []interface{}) *WorkspacePermissions {
	if len(perms) == 0 || perms[0] == nil {
		return nil
	}

	permMap := perms[0].(map[string]interface{})
	result := &WorkspacePermissions{}

	if v, ok := permMap["library_write"]; ok && len(v.([]interface{})) > 0 {
		if writeMap := v.([]interface{})[0].(map[string]interface{}); writeMap != nil {
			result.LibraryWrite = &PermissionLevel{}
			if users, ok := writeMap["users"]; ok {
				result.LibraryWrite.Users = expandStringList(users.(*schema.Set).List())
			}
			if groups, ok := writeMap["groups"]; ok {
				result.LibraryWrite.Groups = expandStringList(groups.(*schema.Set).List())
			}
		}
	}

	if v, ok := permMap["library_read"]; ok && len(v.([]interface{})) > 0 {
		if readMap := v.([]interface{})[0].(map[string]interface{}); readMap != nil {
			result.LibraryRead = &PermissionLevel{}
			if users, ok := readMap["users"]; ok {
				result.LibraryRead.Users = expandStringList(users.(*schema.Set).List())
			}
			if groups, ok := readMap["groups"]; ok {
				result.LibraryRead.Groups = expandStringList(groups.(*schema.Set).List())
			}
		}
	}

	return result
}

// flattenWorkspacePermissions converts WorkspacePermissions to Terraform schema data
func flattenWorkspacePermissions(perms *WorkspacePermissions) []interface{} {
	if perms == nil {
		return []interface{}{}
	}

	result := make(map[string]interface{})

	if perms.LibraryWrite != nil {
		writePerms := make(map[string]interface{})
		if len(perms.LibraryWrite.Users) > 0 {
			writePerms["users"] = flattenStringSet(perms.LibraryWrite.Users)
		}
		if len(perms.LibraryWrite.Groups) > 0 {
			writePerms["groups"] = flattenStringSet(perms.LibraryWrite.Groups)
		}
		if len(writePerms) > 0 {
			result["library_write"] = []interface{}{writePerms}
		}
	}

	if perms.LibraryRead != nil {
		readPerms := make(map[string]interface{})
		if len(perms.LibraryRead.Users) > 0 {
			readPerms["users"] = flattenStringSet(perms.LibraryRead.Users)
		}
		if len(perms.LibraryRead.Groups) > 0 {
			readPerms["groups"] = flattenStringSet(perms.LibraryRead.Groups)
		}
		if len(readPerms) > 0 {
			result["library_read"] = []interface{}{readPerms}
		}
	}

	if len(result) == 0 {
		return []interface{}{}
	}

	return []interface{}{result}
}
