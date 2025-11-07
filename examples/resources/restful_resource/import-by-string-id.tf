import {
  to = restful_resource.test
  id = jsonencode({
    id = "/posts/1"
    path = "/posts"
    body = {
      foo = null
    }
    header = {
      key = "val"
    }
    query = {
      x = ["y"]
    }
  })
}
