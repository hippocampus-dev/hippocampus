use std::sync::Arc;

wasmtime::component::bindgen!({
    path: "wit/tokenizer.wit",
    world: "tokenizer",
    async: true
});

struct PluginState {
    wasi: wasmtime_wasi::WasiCtx,
    table: wasmtime::component::ResourceTable,
}

impl wasmtime_wasi::WasiView for PluginState {
    fn ctx(&mut self) -> &mut wasmtime_wasi::WasiCtx {
        &mut self.wasi
    }

    fn table(&mut self) -> &mut wasmtime::component::ResourceTable {
        &mut self.table
    }
}

#[derive(Clone)]
pub struct WasmTokenizer {
    engine: wasmtime::Engine,
    component: Arc<wasmtime::component::Component>,
    linker: Arc<wasmtime::component::Linker<PluginState>>,
}

impl WasmTokenizer {
    pub fn from_file<P>(path: P) -> Result<Self, error::Error>
    where
        P: AsRef<std::path::Path>,
    {
        let mut configuration = wasmtime::Config::new();
        configuration.async_support(true);
        configuration.wasm_component_model(true);
        configuration.consume_fuel(true);

        let engine = wasmtime::Engine::new(&configuration)
            .map_err(|e| error::error!("Failed to create WASM engine: {e}"))?;

        let component_bytes = std::fs::read(path.as_ref())
            .map_err(|e| error::error!("Failed to read WASM file: {e}"))?;

        let component = wasmtime::component::Component::from_binary(&engine, &component_bytes)
            .map_err(|e| error::error!("Failed to compile component: {e}"))?;

        let mut linker = wasmtime::component::Linker::new(&engine);
        wasmtime_wasi::add_to_linker_async(&mut linker)
            .map_err(|e| error::error!("Failed to add WASI to linker: {e}"))?;

        Ok(Self {
            engine,
            component: Arc::new(component),
            linker: Arc::new(linker),
        })
    }
}

impl crate::tokenizer::Tokenizer for WasmTokenizer {
    #[cfg_attr(feature = "tracing", tracing::instrument(skip(self, content)))]
    fn tokenize<S>(&mut self, content: S) -> Result<Vec<String>, error::Error>
    where
        S: AsRef<str> + std::fmt::Debug,
    {
        let s = content.as_ref().to_string();

        let handle = tokio::runtime::Handle::current();
        handle.block_on(async {
            let wasi = wasmtime_wasi::WasiCtxBuilder::new().build();
            let state = PluginState {
                wasi,
                table: wasmtime::component::ResourceTable::new(),
            };
            let mut store = wasmtime::Store::new(&self.engine, state);

            store
                .set_fuel(10_000_000)
                .map_err(|e| error::error!("Failed to set fuel: {e}"))?;

            let instance = Tokenizer::instantiate_async(&mut store, &self.component, &self.linker)
                .await
                .map_err(|e| error::error!("Failed to instantiate: {e}"))?;

            let tokens = instance
                .call_tokenize(&mut store, &s)
                .await
                .map_err(|e| error::error!("Tokenize call failed: {e}"))?;

            Ok(tokens)
        })
    }
}
