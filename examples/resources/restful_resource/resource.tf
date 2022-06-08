resource "restful_resource" "rg" {
  path = format("/subscriptions/%s/resourceGroups/%s", var.subscription_id, "example")
  query = {
    api-version = ["2020-06-01"]
  }
  create_method = "PUT"
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
      foo = "bar"
    }
  })
}
