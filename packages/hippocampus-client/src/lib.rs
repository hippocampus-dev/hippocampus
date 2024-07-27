pub mod hello_world {
    tonic::include_proto!("helloworld");
}

struct Client {
    client: hello_world::greeter_client::GreeterClient<tonic::transport::Channel>,
}

impl Client {
    pub async fn new() -> Result<Self, Box<dyn std::error::Error + Send + Sync + 'static>> {
        let channel = tonic::transport::Channel::from_static("http://127.0.0.1:8080")
            .connect_timeout(std::time::Duration::from_millis(10))
            .connect()
            .await?;
        let client = hello_world::greeter_client::GreeterClient::new(channel);
        Ok(Self { client })
    }

    pub async fn say_hello(
        &mut self,
        name: String,
    ) -> Result<String, Box<dyn std::error::Error + Send + Sync + 'static>> {
        let request = tonic::Request::new(hello_world::HelloRequest { name });
        let response = self.client.say_hello(request).await?.into_inner();
        Ok(response.message)
    }
}
