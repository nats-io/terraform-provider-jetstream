// Creates a Stream Template called JS_1H that creates new streams
// for any traffic under JS.1H.*, if you publish to JS.1H.hello it
// will create a stream called JS_1H_hello
provider "jetstream" {
  servers = "connect.ngs.global"
  credentials = "ngs_jetstream_admin.creds"
}

resource "jetstream_stream_template" "ONE_HOUR" {
  name        = "JS_1H"
  subjects    = ["JS.1H.*"]
  storage     = "file"
  max_age     = 60 * 60
  max_streams = 60
}
