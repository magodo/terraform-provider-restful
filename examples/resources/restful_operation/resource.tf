resource "restful_operation" "register_rp" {
  path = format("/subscriptions/%s/providers/Microsoft.ProviderHub/register", var.subscription_id)
  query = {
    api-version = ["2014-04-01-preview"]
  }
  method = "POST"
  poll = {
    url_locator    = format("exact[/subscriptions/%s/providers/Microsoft.ProviderHub?api-version=2014-04-01-preview]", var.subscription_id)
    status_locator = "body[registrationState]"
    status = {
      success = "Registered"
      pending = ["Registering"]
    }
  }
}
