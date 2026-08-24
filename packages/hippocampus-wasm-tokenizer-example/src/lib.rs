wit_bindgen::generate!({
    world: "tokenizer",
    path: "wit/world.wit",
});

struct Component;

impl Guest for Component {
    fn tokenize(content: String) -> Vec<String> {
        content
            .split(|character: char| !character.is_alphanumeric())
            .filter(|string| !string.is_empty())
            .map(|string| string.to_lowercase())
            .collect()
    }
}

export!(Component);
