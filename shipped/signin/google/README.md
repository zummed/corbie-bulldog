Google: a person signs in to the screens with their Google account, when the office writes its address under their name.

The sign-in page shows "Sign in with Google". Bulldog sends the browser to Google, with a challenge and a nonce of its own, and Google sends it back to `<office url>/signin/<name>/back` with a code. Bulldog starts this machine with the code, and the machine, which alone holds the client's secret, takes the code to Google, reads the ID token Google answers and checks it: it is for the office's client, Google's, not expired, carries the nonce Bulldog gave, and says Google verified the address. Its answer is the address, in small letters, and Bulldog signs in the one person the office writes with that address under the method's name. An account the office does not write signs nobody in, whatever its domain.

## Needs

- `GOOGLE_CLIENT_SECRET`, the OAuth client's secret, for `oauth2.googleapis.com`. Only this machine's job holds it; Bulldog never sees it.

## Settings

- `from`: `./.bulldog/signin/google`, the folder it runs, as Bulldog ships it.
- `client_id`: the OAuth client's ID, from the Google Cloud console's Credentials page. It is no secret: Google shows it to every browser it sends back.
- `authorize`: `https://accounts.google.com/o/oauth2/v2/auth`. Google's authorization endpoint, where Bulldog sends the browser.
- `scope`: `openid email`. What Google is asked for.
- `label`: `Google`. What the sign-in page calls it.

## Set up at Google

1. In the Google Cloud console, under APIs & Services, set up the OAuth consent screen: internal, for a Google Workspace of your own, or external, with the people who sign in as its test users or the app published.
2. Under Credentials, create an OAuth client ID of the type Web application, and add as its authorized redirect URI `<office url>/signin/<name>/back`, exactly as the server's log names it at its start, with the office's `--url`, as `https://bulldog.example.com/signin/google/back`. Without `--url` the office is reached at `http://127.0.0.1:7410`, which Google takes only from the computer the server runs on.
3. Write its client ID as `client_id` in the entry, and keep its secret in Corbie: `corbie controller secret set GOOGLE_CLIENT_SECRET`.
4. Write each person's Google address under their name, as `google: sam@example.com`.

## Leaves out

- An address Google has not verified.
- Anyone the office does not write: Google's yes is only who the person is, never that they may come in.

## Example

```yaml
# in office.yaml
signin:
  google:
    from: ./.bulldog/signin/google
    client_id: 1234567890-abc.apps.googleusercontent.com
everyone:
  sam: { name: Sam, google: sam@example.com }
```

## Changed

- 0.3.0: first shipped.
