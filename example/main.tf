terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}

# --- SCENARIO 1: Happy Path (Supported) ---
# This resource should fully map, price, and pass policy.
resource "aws_instance" "production_web" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t3.medium" # Mapped & Priced in Stub

  # Root Block Device maps to EBS Storage
  root_block_device {
    volume_size = 50 # 50 GB
    volume_type = "gp2"
  }

  tags = {
    Name        = "Prod Web"
    Environment = "Production"
    CostCenter  = "101"
  }
}

# --- SCENARIO 2: Policy Block (Budget Exceeded) ---
# Uncommenting this should trigger a "Budget Exceeded" error
# because 10x m5.2xlarge will blow the $1200 limit.
/*
resource "aws_instance" "expensive_cluster" {
  count         = 5
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "m5.2xlarge"
}
*/

# --- SCENARIO 3: Safety Check (Fail-Closed) ---
# Uncommenting this should trigger an "INCOMPLETE" status
# because we don't have a price for 'p4d.24xlarge' in the stub.
/*
resource "aws_instance" "price_missing" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "p4d.24xlarge"
}
*/
