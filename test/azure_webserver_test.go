package test

import (
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/azure"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
)

// You normally want to run this under a separate "Testing" subscription
// For lab purposes you will use your assigned subscription under the Cloud Dev/Ops program tenant
var subscriptionID string = "4a9683b4-eb05-4bdb-a62b-b9666ace2102"

func TestAzureLinuxVMCreation(t *testing.T) {
	terraformOptions := &terraform.Options{
		// The path to where our Terraform code is located
		TerraformDir: "../",
		// Override the default terraform variables
		Vars: map[string]interface{}{
			"labelPrefix": "yashrajput",
		},
	}

	// Set total timeout to 5 minutes (300 seconds)
	timeout := 5 * time.Minute
	timeoutChan := time.After(timeout)

	// Run Terraform apply and destroy within timeout
	doneChan := make(chan bool)
	go func() {
		// Apply Terraform changes
		terraform.InitAndApply(t, terraformOptions)

		// Get Terraform output values
		vmName := terraform.Output(t, terraformOptions, "vm_name")
		resourceGroupName := terraform.Output(t, terraformOptions, "resource_group_name")

		// Confirm VM exists
		assert.True(t, azure.VirtualMachineExists(t, vmName, resourceGroupName, subscriptionID))

		// Destroy Terraform resources
		terraform.Destroy(t, terraformOptions)

		// Signal completion
		doneChan <- true
	}()

	// Wait for completion or timeout
	select {
	case <-doneChan:
		t.Log("Test completed successfully within timeout.")
	case <-timeoutChan:
		t.Fatal("Test timeout reached! Azure is taking too long.")
	}
}
