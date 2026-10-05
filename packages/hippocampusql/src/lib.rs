//! A query language parser for hippocampus.
//!
//! # Grammar
//!
//! ```text
//! <term> := [^"]+
//! <phrase> := '"' <term> '"'
//! <term_or_phrase> := <term> | <phrase>
//! <operation> := <term_or_phrase> (" AND " | " OR " | " NOT ") (<term_or_phrase> | <operation>)
//! ```
//!
//! # Examples
//!
//! ```rust,ignore
//! let (_, query) = hippocampusql::parse("hello AND world")?;
//! ```

use nom::Finish;

#[derive(Clone, Debug, PartialEq)]
pub enum Query {
    Term(Term),
    Phrase(Phrase),
    Operation(Box<Operation>),
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Term(pub String);

impl std::fmt::Display for Term {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.0)
    }
}

const AND_TAG: &str = " AND ";
const OR_TAG: &str = " OR ";
const NOT_TAG: &str = " NOT ";

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Phrase(pub Term);

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Operator {
    AND,
    OR,
    NOT,
}

#[derive(Clone, Debug, PartialEq)]
pub struct Operation {
    pub operator: Operator,
    pub left: Query,
    pub right: Query,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ParseError(String, nom::error::ErrorKind);

impl std::error::Error for ParseError {}
impl std::fmt::Display for ParseError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "parse error at {}: {:?}", self.0, self.1)
    }
}

/// Return a struct from hippocampusql query
///
/// # Arguments
///
/// - `input` - A query of hippocampusql
///
/// # Example
///
/// ```
/// use hippocampusql::*;
///
/// assert_eq!(parse("foo"), Ok(("", Query::Term(Term("foo".to_string())))));
/// ```
pub fn parse(input: &str) -> Result<(&str, Query), ParseError> {
    nom::branch::alt((operation, phrase, term))(input)
        .finish()
        .map_err(|e| {
            let nom::error::Error { input, code } = e;
            ParseError(input.to_string(), code)
        })
}

fn is_valid_character(c: char) -> bool {
    c != '"'
}

fn term(input: &str) -> nom::IResult<&str, Query> {
    let (input, word) = nom::branch::alt((
        nom::bytes::complete::take_until(AND_TAG),
        nom::bytes::complete::take_until(OR_TAG),
        nom::bytes::complete::take_until(NOT_TAG),
        nom::bytes::complete::take_while1(is_valid_character),
    ))(input)?;
    Ok((input, Query::Term(Term(word.to_string()))))
}

fn phrase(input: &str) -> nom::IResult<&str, Query> {
    let (input, word) = nom::sequence::delimited(
        nom::character::complete::char('"'),
        term,
        nom::character::complete::char('"'),
    )(input)?;
    match word {
        Query::Term(term) => Ok((input, Query::Phrase(Phrase(term)))),
        _ => panic!(),
    }
}

fn and(input: &str) -> nom::IResult<&str, Operator> {
    let (input, _) = nom::bytes::complete::tag(AND_TAG)(input)?;
    Ok((input, Operator::AND))
}

fn or(input: &str) -> nom::IResult<&str, Operator> {
    let (input, _) = nom::bytes::complete::tag(OR_TAG)(input)?;
    Ok((input, Operator::OR))
}

fn not(input: &str) -> nom::IResult<&str, Operator> {
    let (input, _) = nom::bytes::complete::tag(NOT_TAG)(input)?;
    Ok((input, Operator::NOT))
}

fn operation(input: &str) -> nom::IResult<&str, Query> {
    let (input, left) = nom::branch::alt((phrase, term))(input)?;
    let (input, operator) = nom::branch::alt((and, or, not))(input)?;
    let (input, right) = nom::branch::alt((operation, phrase, term))(input)?;
    Ok((
        input,
        Query::Operation(Box::new(Operation {
            operator,
            left,
            right,
        })),
    ))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn empty() {
        dbg!(&parse(""));
        assert_eq!(
            parse(""),
            Err(ParseError(
                "".to_string(),
                nom::error::ErrorKind::TakeWhile1
            ))
        );
    }

    #[test]
    fn single_word() {
        assert_eq!(parse("foo"), Ok(("", Query::Term(Term("foo".to_string())))));
    }

    #[test]
    fn multibyte_word() {
        assert_eq!(
            parse("マルチバイト"),
            Ok(("", Query::Term(Term("マルチバイト".to_string()))))
        );
    }

    #[test]
    fn multiple_word() {
        assert_eq!(
            parse("foo bar"),
            Ok(("", Query::Term(Term("foo bar".to_string()))))
        );
    }

    #[test]
    fn single_word_in_phrase() {
        assert_eq!(
            parse("\"foo\""),
            Ok(("", Query::Phrase(Phrase(Term("foo".to_string())))))
        );
    }

    #[test]
    fn multiple_word_in_phrase() {
        assert_eq!(
            parse("\"foo bar\""),
            Ok(("", Query::Phrase(Phrase(Term("foo bar".to_string())))))
        );
    }

    #[test]
    fn and_operator() {
        assert_eq!(
            parse("foo AND bar"),
            Ok((
                "",
                Query::Operation(Box::new(Operation {
                    operator: Operator::AND,
                    left: Query::Term(Term("foo".to_string())),
                    right: Query::Term(Term("bar".to_string())),
                }))
            ))
        );
    }

    #[test]
    fn or_operator() {
        assert_eq!(
            parse("foo OR bar"),
            Ok((
                "",
                Query::Operation(Box::new(Operation {
                    operator: Operator::OR,
                    left: Query::Term(Term("foo".to_string())),
                    right: Query::Term(Term("bar".to_string())),
                }))
            ))
        );
    }

    #[test]
    fn not_operator() {
        assert_eq!(
            parse("foo NOT bar"),
            Ok((
                "",
                Query::Operation(Box::new(Operation {
                    operator: Operator::NOT,
                    left: Query::Term(Term("foo".to_string())),
                    right: Query::Term(Term("bar".to_string())),
                }))
            ))
        );
    }

    #[test]
    fn operation_with_multiple_word() {
        assert_eq!(
            parse("foo AND bar baz"),
            Ok((
                "",
                Query::Operation(Box::new(Operation {
                    operator: Operator::AND,
                    left: Query::Term(Term("foo".to_string())),
                    right: Query::Term(Term("bar baz".to_string()))
                }))
            ))
        );
    }

    #[test]
    fn operation_with_phrase() {
        assert_eq!(
            parse("foo AND \"bar baz\""),
            Ok((
                "",
                Query::Operation(Box::new(Operation {
                    operator: Operator::AND,
                    left: Query::Term(Term("foo".to_string())),
                    right: Query::Phrase(Phrase(Term("bar baz".to_string())))
                }))
            ))
        );
    }

    #[test]
    fn multiple_operation() {
        assert_eq!(
            parse("foo AND bar OR baz"),
            Ok((
                "",
                Query::Operation(Box::new(Operation {
                    operator: Operator::AND,
                    left: Query::Term(Term("foo".to_string())),
                    right: Query::Operation(Box::new(Operation {
                        operator: Operator::OR,
                        left: Query::Term(Term("bar".to_string())),
                        right: Query::Term(Term("baz".to_string())),
                    })),
                }))
            ))
        );
    }
}
