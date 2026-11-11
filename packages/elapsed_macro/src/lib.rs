#[proc_macro_attribute]
pub fn elapsed(
    _attr: proc_macro::TokenStream,
    item: proc_macro::TokenStream,
) -> proc_macro::TokenStream {
    let mut ast = syn::parse_macro_input!(item as syn::ItemFn);
    let ident = &ast.sig.ident;

    let body = ast.block.as_ref();
    let body: syn::Block = syn::parse_quote! {{
        if let Some(start) = elapsed::BASE_TIME.get() {
            println!("#{} {:?} later in {}:{}", stringify!(#ident), start.elapsed(), file!(), line!());
        } else {
            let _ = elapsed::BASE_TIME.set(elapsed::SerializableTime::new(std::time::Instant::now()));
        }

        #body
    }};

    *ast.block = body;

    let generated = quote::quote! {
        #ast
    };

    generated.into()
}
