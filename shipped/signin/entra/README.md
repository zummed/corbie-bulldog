Microsoft Entra ID: a person signs in to the screens with their work account in the office's directory, when the office writes its sign-in name under their name.

The sign-in page shows "Sign in with Microsoft". Bulldog sends the browser to the directory, with a challenge and a nonce of its own, and the directory sends it back to `<office url>/signin/<name>/back` with a code. Bulldog starts this machine with the code, and the machine, which alone holds the app's secret, takes the code to Microsoft, reads the ID token Microsoft answers and checks it: it is for the office's app, of the directory `tenant` names, Microsoft's, not expired, and carries the nonce Bulldog gave. Its answer is the account's sign-in name, `preferred_username`, or its `email` where it has none, in small letters, and Bulldog signs in the one person the office writes with that name under the method's name. An account of another directory, and one the office does not write, sign nobody in.

## Needs

- `ENTRA_CLIENT_SECRET`, the app's client secret, for `login.microsoftonline.com`. Only this machine's job holds it; Bulldog never sees it.

## Settings

- `from`: `./.bulldog/signin/entra`, the folder it runs, as Bulldog ships it.
- `tenant`: the directory's ID, the Directory (tenant) ID on the app's Overview page, a GUID.
- `client_id`: the app's Application (client) ID, on its Overview page. It is no secret.
- `authorize`: `https://login.microsoftonline.com/{tenant}/oauth2/v2.0/authorize`. Where Bulldog sends the browser; `{tenant}` is the tenant setting.
- `scope`: `openid profile email`. What the directory is asked for.
- `label`: `Microsoft`. What the sign-in page calls it.

## Set up at Microsoft

1. In the Microsoft Entra admin center, under App registrations, make a new registration, for accounts in this organizational directory only.
2. Give it, under Authentication, the platform Web, with the redirect URI `<office url>/signin/<name>/back`, exactly as the server's log names it at its start, with the office's `--url`, as `https://bulldog.example.com/signin/entra/back`. Without `--url` the office is reached at `http://127.0.0.1:7410`, which Microsoft takes only as `localhost`, from the computer the server runs on.
3. Under Certificates & secrets, make a client secret, and keep its value in Corbie: `corbie controller secret set ENTRA_CLIENT_SECRET`. It expires: make a new one before it does.
4. Write the Directory (tenant) ID as `tenant` and the Application (client) ID as `client_id` in the entry.
5. Write each person's sign-in name under their name, as `entra: al@corp.example`.

## Leaves out

- Any account of another directory, and any account the office does not write: the directory's yes is only who the person is, never that they may come in.

## Example

```yaml
# in office.yaml
signin:
  entra:
    from: ./.bulldog/signin/entra
    tenant: 72f988bf-86f1-41af-91ab-2d7cd011db47
    client_id: 5f3c1a2e-0b7d-4e8a-9c61-2a4b8d0e7f19
everyone:
  al: { name: Al, entra: al@corp.example }
```

## Changed

- 0.3.0: first shipped.
