# Import existing JetStream resources into state

When migrating to the JetStream Terraform provider, it may be the case hat some resources have already been
created manually within the NATS JetStream cluster. It's possible to import these resources into the Terraform state
and start managing them using Terraform. 

