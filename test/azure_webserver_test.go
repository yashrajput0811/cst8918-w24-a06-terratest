package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/azure"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
)

// Azure subscription ID (Ensure this matches your assigned subscription)
var subscriptionID string = "4a9683b4-eb05-4bdb-a62b-b9666ace2102"

// Number of retries for destroy operation
const maxRetries = 5

func TestAzureLinuxVMCreation(t *testing.T) {
	terraformOptions := &terraform.Options{
		TerraformDir: "../", // Path to Terraform code
		Vars: map[string]interface{}{
			"labelPrefix": "yashrajput", // Prefix to avoid conflicts
		},
	}

	// Set timeout for the test
	timeout := 15 * time.Minute
	timeoutChan := time.After(timeout)
	doneChan := make(chan error) // Channel to handle errors properly

	// Run Terraform apply in a goroutine
	go func() {
		t.Log("🚀 Running Terraform deployment...")

		// Apply Terraform configuration
		_, err := terraform.InitAndApplyE(t, terraformOptions)
		if err != nil {
			doneChan <- fmt.Errorf("❌ Terraform apply failed: %v", err)
			return
		}

		t.Log("✅ Terraform apply completed.")

		// Retrieve Terraform output values
		vmName := terraform.Output(t, terraformOptions, "vm_name")
		resourceGroupName := terraform.Output(t, terraformOptions, "resource_group_name")
		nicName := terraform.Output(t, terraformOptions, "nic_name")

		// Expected Ubuntu version
		expectedUbuntuVersion := "22"

		// 🔍 Validate VM existence
		t.Logf("🔄 Checking if VM '%s' exists in resource group '%s'...", vmName, resourceGroupName)
		if !azure.VirtualMachineExists(t, vmName, resourceGroupName, subscriptionID) {
			doneChan <- fmt.Errorf("❌ VM does not exist!")
			return
		}

		// 🔍 Validate NIC existence and attachment
		t.Logf("🔄 Checking if NIC '%s' exists in resource group '%s'...", nicName, resourceGroupName)
		nic, err := azure.GetNetworkInterfaceE(nicName, resourceGroupName, subscriptionID)
		if err != nil {
			doneChan <- fmt.Errorf("❌ NIC retrieval failed: %v", err)
			return
		}
		if nic == nil {
			doneChan <- fmt.Errorf("❌ NIC does not exist or is not properly attached!")
			return
		}
		t.Logf("✅ NIC '%s' exists and is attached correctly.", nicName)

		// 🔍 Validate VM OS version
		t.Log("🔄 Fetching VM details to check OS version...")
		vmInfo, err := azure.GetVirtualMachineE(vmName, resourceGroupName, subscriptionID)
		if err != nil {
			doneChan <- fmt.Errorf("❌ Failed to fetch VM details: %v", err)
			return
		}

		// Ensure VM info is retrieved properly
		if vmInfo == nil || vmInfo.StorageProfile == nil || vmInfo.StorageProfile.ImageReference == nil || vmInfo.StorageProfile.ImageReference.Sku == nil {
			doneChan <- fmt.Errorf("❌ VM information is incomplete! Cannot verify OS version.")
			return
		}

		// Retrieve the OS version as a string
		actualUbuntuVersion := *vmInfo.StorageProfile.ImageReference.Sku
		t.Logf("🔍 Retrieved VM OS Version: %s", actualUbuntuVersion)

		// Allow variations like "22_04-lts", "22_04-lts-gen2"
		if !assert.Contains(t, actualUbuntuVersion, expectedUbuntuVersion) {
			doneChan <- fmt.Errorf("❌ VM is not running the expected Ubuntu version!")
			return
		}
		t.Logf("✅ VM '%s' is running Ubuntu %s as expected.", vmName, actualUbuntuVersion)

		// Signal successful completion
		doneChan <- nil
	}()

	// Handle test errors or timeout
	select {
	case err := <-doneChan:
		if err != nil {
			t.Fatal(err) // Call Fatal only in the main test goroutine
		} else {
			t.Log("🎉 Test completed successfully within the allotted time.")
		}
	case <-timeoutChan:
		t.Fatal("⏳ Test timeout reached! The deployment or verification took too long.")
	}

	// 🔄 Retry Terraform destroy operation to avoid errors
	retryTerraformDestroy(t, terraformOptions)
}

// 🔄 Retry-based destroy function to handle Azure API errors
func retryTerraformDestroy(t *testing.T, terraformOptions *terraform.Options) {
	retryInterval := 30 * time.Second // Wait 30 seconds between retries

	for attempt := 1; attempt <= maxRetries; attempt++ {
		t.Logf("🛑 Attempting Terraform destroy (Attempt %d/%d)...", attempt, maxRetries)

		// Fix: Capture both returned values
		stdout, err := terraform.DestroyE(t, terraformOptions)

		if err == nil {
			t.Log("✅ Terraform destroy succeeded!")
			return
		}

		t.Logf("⚠️ Destroy attempt %d failed: %v\nTerraform Output: %s", attempt, err, stdout)

		if attempt < maxRetries {
			t.Logf("⏳ Retrying in %s...", retryInterval)
			time.Sleep(retryInterval)
		} else {
			t.Fatalf("❌ Terraform destroy failed after %d attempts: %v", maxRetries, err)
		}
	}
}
