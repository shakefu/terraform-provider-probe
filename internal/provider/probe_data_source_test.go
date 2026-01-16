// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"probe": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates that required environment variables are set for
// acceptance tests to run. Tests use LocalStack or real AWS credentials.
func testAccPreCheck(t *testing.T) {
	// If LocalStack is running, use it
	if localStackRunning() {
		return
	}
	// Otherwise check for AWS credentials
	if os.Getenv("AWS_PROFILE") == "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("Skipping acceptance test: LocalStack not running and AWS credentials not configured")
	}
}

// localStackRunning checks if LocalStack is available at localhost:4566
func localStackRunning() bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://localhost:4566/_localstack/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// useLocalStack returns true if tests should use LocalStack
func useLocalStack() bool {
	return localStackRunning()
}

func TestAccProbeDataSource_notFound(t *testing.T) {
	config := testAccProbeDataSourceConfig_notFound
	if useLocalStack() {
		config = testAccProbeDataSourceConfig_notFound_localstack
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.probe.test", "exists", "false"),
					resource.TestCheckNoResourceAttr("data.probe.test", "arn"),
					resource.TestCheckNoResourceAttr("data.probe.test", "properties"),
				),
			},
		},
	})
}

const testAccProbeDataSourceConfig_notFound = `
data "probe" "test" {
  type = "AWS::DynamoDB::Table"
  id   = "nonexistent-table-that-does-not-exist-12345"
}
`

const testAccProbeDataSourceConfig_notFound_localstack = `
provider "probe" {
  localstack = true
}

data "probe" "test" {
  type = "AWS::DynamoDB::Table"
  id   = "nonexistent-table-that-does-not-exist-12345"
}
`

func TestAccProbeDataSource_existingResource(t *testing.T) {
	config := testAccProbeDataSourceConfig_existing
	checks := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("data.probe.test", "exists", "true"),
		resource.TestCheckResourceAttrSet("data.probe.test", "arn"),
		resource.TestCheckResourceAttrSet("data.probe.test", "properties.%"),
	)

	if useLocalStack() {
		config = testAccProbeDataSourceConfig_existing_localstack
		// With native SDK, we get full properties including ARN
		checks = resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.probe.test", "exists", "true"),
			resource.TestCheckResourceAttrSet("data.probe.test", "arn"),
			resource.TestCheckResourceAttrSet("data.probe.test", "properties.%"),
			resource.TestCheckResourceAttr("data.probe.test", "properties.TableName", "probe-acceptance-test-table"),
		)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: "~> 5.0",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  checks,
			},
		},
	})
}

const testAccProbeDataSourceConfig_existing = `
provider "aws" {}

resource "aws_dynamodb_table" "test" {
  name         = "probe-acceptance-test-table"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "id"

  attribute {
    name = "id"
    type = "S"
  }
}

data "probe" "test" {
  type = "AWS::DynamoDB::Table"
  id   = aws_dynamodb_table.test.name

  depends_on = [aws_dynamodb_table.test]
}
`

const testAccProbeDataSourceConfig_existing_localstack = `
provider "probe" {
  localstack = true
}

provider "aws" {
  region                      = "us-east-1"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  access_key                  = "test"
  secret_key                  = "test"

  endpoints {
    dynamodb = "http://localhost:4566"
  }
}

resource "aws_dynamodb_table" "test" {
  name         = "probe-acceptance-test-table"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "id"

  attribute {
    name = "id"
    type = "S"
  }

  tags = {
    Environment = "test"
    Project     = "probe-provider"
  }
}

data "probe" "test" {
  type = "AWS::DynamoDB::Table"
  id   = aws_dynamodb_table.test.name

  depends_on = [aws_dynamodb_table.test]
}
`

func TestAccProbeDataSource_terraformTypeSyntax(t *testing.T) {
	config := testAccProbeDataSourceConfig_terraformType
	if useLocalStack() {
		config = testAccProbeDataSourceConfig_terraformType_localstack
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.probe.test", "exists", "false"),
				),
			},
		},
	})
}

// Tests that Terraform-style resource types (aws_dynamodb_table) are correctly
// resolved to the appropriate native SDK prober.
const testAccProbeDataSourceConfig_terraformType = `
data "probe" "test" {
  type = "aws_dynamodb_table"
  id   = "nonexistent-table-terraform-syntax-12345"
}
`

const testAccProbeDataSourceConfig_terraformType_localstack = `
provider "probe" {
  localstack = true
}

data "probe" "test" {
  type = "aws_dynamodb_table"
  id   = "nonexistent-table-terraform-syntax-12345"
}
`

func TestAccProbeDataSource_tagsReturned(t *testing.T) {
	if !useLocalStack() {
		t.Skip("Tags test only runs against LocalStack")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: "~> 5.0",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: testAccProbeDataSourceConfig_tags_localstack,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.probe.test", "exists", "true"),
					resource.TestCheckResourceAttrSet("data.probe.test", "arn"),
					resource.TestCheckResourceAttr("data.probe.test", "properties.Tags.%", "2"),
					resource.TestCheckResourceAttr("data.probe.test", "properties.Tags.Environment", "test"),
					resource.TestCheckResourceAttr("data.probe.test", "properties.Tags.Project", "probe-tags-test"),
				),
			},
		},
	})
}

const testAccProbeDataSourceConfig_tags_localstack = `
provider "probe" {
  localstack = true
}

provider "aws" {
  region                      = "us-east-1"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  access_key                  = "test"
  secret_key                  = "test"

  endpoints {
    dynamodb = "http://localhost:4566"
  }
}

resource "aws_dynamodb_table" "test" {
  name         = "probe-tags-test-table"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "id"

  attribute {
    name = "id"
    type = "S"
  }

  tags = {
    Environment = "test"
    Project     = "probe-tags-test"
  }
}

data "probe" "test" {
  type = "aws_dynamodb_table"
  id   = aws_dynamodb_table.test.name

  depends_on = [aws_dynamodb_table.test]
}
`
