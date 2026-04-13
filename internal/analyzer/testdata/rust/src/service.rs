use crate::model::User;
use super::model;

pub trait Repository {
    fn find_by_id(&self, id: u64) -> Option<User>;
    fn save(&self, user: &User) -> bool;
}

pub trait Notifier {
    fn notify(&self, message: &str);
}

pub struct UserService {
    name: String,
}

pub(crate) struct InternalCache {
    size: usize,
}

struct PrivateHelper;

impl UserService {
    pub fn new() -> Self {
        UserService { name: String::new() }
    }

    pub fn create_user(&self, name: &str) -> User {
        User { id: 1, name: name.to_string() }
    }
}

impl Repository for UserService {
    fn find_by_id(&self, id: u64) -> Option<User> {
        None
    }

    fn save(&self, user: &User) -> bool {
        true
    }
}

pub async fn handle_request(svc: &UserService) -> Result<(), String> {
    Ok(())
}

fn internal_helper() {}
