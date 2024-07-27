#[proc_macro_attribute]
pub fn elapsed(
    _attr: proc_macro::TokenStream,
    item: proc_macro::TokenStream,
) -> proc_macro::TokenStream {
    let mut ast = syn::parse_macro_input!(item as syn::ItemFn);
    let ident = &ast.sig.ident;

    let body = ast.block.as_ref();
    let body: syn::Block = syn::parse_quote! {{
        if let Ok(start) = std::env::var("ELAPSED::BASE_TIME") {
            let s: elapsed::SerializableTime = serde_json::from_str(&start).unwrap();
            println!("#{} {:?} later in {}:{}", stringify!(#ident), s.elapsed(), file!(), line!());
        } else {
            std::env::set_var("ELAPSED::BASE_TIME", serde_json::to_string(&elapsed::SerializableTime::new(std::time::Instant::now())).unwrap_or("".to_string()));
        }

        #body
    }};

    *ast.block = body;

    let gen = quote::quote! {
        #ast
    };

    gen.into()
}
