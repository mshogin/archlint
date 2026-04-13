use std::fmt;

pub struct User {
    pub id: u64,
    pub name: String,
}

pub struct Config {
    pub debug: bool,
}

pub enum Status {
    Active,
    Inactive,
    Pending(String),
}

pub enum Role {
    Admin,
    User,
    Guest,
}

enum PrivateState {
    Open,
    Closed,
}

pub trait Displayable {
    fn display(&self) -> String;
}

impl Displayable for User {
    fn display(&self) -> String {
        format!("{}: {}", self.id, self.name)
    }
}

impl fmt::Display for Status {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "status")
    }
}
