package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccOpensearchWorkspace(t *testing.T) {
	provider := Provider()
	diags := provider.Configure(context.Background(), &terraform.ResourceConfig{})
	if diags.HasError() {
		t.Skipf("err: %#v", diags)
	}

	randomName := "test" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		Providers:    testAccOpendistroProviders,
		CheckDestroy: testAccCheckOpensearchWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccOpenSearchWorkspaceResource(randomName),
				Check: resource.ComposeTestCheckFunc(
					testCheckOpensearchWorkspaceExists("opensearch_workspace.test"),
					resource.TestCheckResourceAttr(
						"opensearch_workspace.test",
						"name",
						randomName,
					),
					resource.TestCheckResourceAttr(
						"opensearch_workspace.test",
						"description",
						"test workspace",
					),
				),
			},
			{
				Config: testAccOpenSearchWorkspaceResourceUpdated(randomName),
				Check: resource.ComposeTestCheckFunc(
					testCheckOpensearchWorkspaceExists("opensearch_workspace.test"),
					resource.TestCheckResourceAttr(
						"opensearch_workspace.test",
						"description",
						"updated test workspace",
					),
				),
			},
		},
	})
}

func testAccCheckOpensearchWorkspaceDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "opensearch_workspace" {
			continue
		}

		meta := testAccOpendistroProvider.Meta()

		var err error
		_, err = resourceOpensearchGetWorkspace(rs.Primary.ID, meta.(*ProviderConf))

		if err != nil {
			return nil // should be not found error
		}

		return fmt.Errorf("Workspace %q still exists", rs.Primary.ID)
	}

	return nil
}

func testCheckOpensearchWorkspaceExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "opensearch_workspace" {
				continue
			}

			meta := testAccOpendistroProvider.Meta()

			var err error
			_, err = resourceOpensearchGetWorkspace(rs.Primary.ID, meta.(*ProviderConf))

			if err != nil {
				return err
			}

			return nil
		}

		return nil
	}
}

func testAccOpenSearchWorkspaceResource(resourceName string) string {
	return fmt.Sprintf(`
resource "opensearch_workspace" "test" {
  name        = "%s"
  description = "test workspace"
}
	`, resourceName)
}

func testAccOpenSearchWorkspaceResourceUpdated(resourceName string) string {
	return fmt.Sprintf(`
resource "opensearch_workspace" "test" {
  name        = "%s"
  description = "updated test workspace"
}
	`, resourceName)
}
