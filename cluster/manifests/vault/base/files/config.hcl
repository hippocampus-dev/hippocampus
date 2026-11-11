disable_mlock = true
ui = false

listener "tcp" {
  tls_disable = 1
  address = "[::]:8200"
  cluster_address = "[::]:8201"
}
storage "file" {
  path = "/data"
}
