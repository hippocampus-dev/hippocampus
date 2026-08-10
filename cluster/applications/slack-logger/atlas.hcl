env "local" {
  src = "file://schema.hcl"
  dev = "docker://mysql/8/dev?charset=utf8mb4"

  migration {
    dir = "file://migrations"
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

env "remote" {
  url = "mysql://${getenv("MYSQL_USER")}:${getenv("MYSQL_PASSWORD")}@${getenv("MYSQL_ADDRESS")}/${getenv("MYSQL_DATABASE")}?charset=utf8mb4&parseTime=true"

  migration {
    dir = "file://migrations"
  }
}
