package outscale

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform/helper/resource"
)

func TestAccOutscaleOAPISnapshotAttributes_Basic(t *testing.T) {
	o := os.Getenv("OUTSCALE_OAPI")

	oapi, err := strconv.ParseBool(o)
	if err != nil {
		oapi = false
	}

	if !oapi {
		t.Skip()
	}

	var snapshotID string
	accountID := os.Getenv("OUTSCALE_ACCOUNT")

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccOutscaleOAPISnapshotAttributesAdditionsConfig(true, accountID),
				Check: resource.ComposeTestCheckFunc(
					testCheckResourceGetAttr("outscale_snapshot.test", "id", &snapshotID),
				),
			},
			resource.TestStep{
				Config: testAccOutscaleOAPISnapshotAttributesRemovalsConfig(true, accountID),
				Check: resource.ComposeTestCheckFunc(
					testCheckResourceGetAttr("outscale_snapshot.test", "id", &snapshotID),
				),
			},
		},
	})
}

func testAccOutscaleOAPISnapshotAttributesAdditionsConfig(includeCreateVolumePermission bool, aid string) string {
	return fmt.Sprintf(`
		resource "outscale_volume" "description_test" {
			subregion_name = "eu-west-2a"
			size = 1
		}

		resource "outscale_snapshot" "test" {
			volume_id = "${outscale_volume.description_test.id}"
			description = "Snapshot Acceptance Test"
		}

		resource "outscale_snapshot_attributes" "self-test" {
			snapshot_id = "${outscale_snapshot.test.id}"
			permissions_to_create_volume_additions = {
					account_ids = ["%s"]
			}  
		}
	`, aid)
}

func testAccOutscaleOAPISnapshotAttributesRemovalsConfig(includeCreateVolumePermission bool, aid string) string {
	return fmt.Sprintf(`
		resource "outscale_volume" "description_test" {
			subregion_name = "eu-west-2a"
			size = 1
		}

		resource "outscale_snapshot" "test" {
			volume_id = "${outscale_volume.description_test.id}"
			description = "Snapshot Acceptance Test"
		}

		resource "outscale_snapshot_attributes" "self-test" {
			snapshot_id = "${outscale_snapshot.test.id}"
			permissions_to_create_volume_removals = {
					account_ids = ["%s"]
			}  
		}
	`, aid)
}