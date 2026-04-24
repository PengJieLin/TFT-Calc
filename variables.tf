variable "instance_name" {
  description = "Value of the EC2 instance's Name tag."
  type        = string
  default     = "learn-terraform"
}

variable "instance_type" {
  description = "The EC2 instance's type."
  type        = string
  default     = "t3.micro"
}

variable "region" {
  description = "Region where resource is located."
  type        = string
  default     = "us-east-2"
}

variable "bucket" {
  description = "S3 bucket where the csv files are stored."
  type        = string
  default     = "tft-calc-s3" 
}