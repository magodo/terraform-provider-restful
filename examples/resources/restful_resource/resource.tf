resource "restful_resource" "rg" {
  path          = format("/subscriptions/%s/resourceGroups/%s", var.subscription_id, "example")
  create_method = "PUT"
  query = {
    api-version = ["2020-06-01"]
  }
  poll_delete = {
    status_locator = "code"
    status = {
      success = "404"
      pending = ["202", "200"]
    }
  }
  body = jsonencode({
    location = "westus"
    tags = {
      foo = "baz"
    }
  })
}
