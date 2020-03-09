#!/usr/bin/env ruby

require "json"
require "pp"

schema = JSON.parse(`terraform providers schema --json`)

schema["provider_schemas"]["jetstream"]["resource_schemas"].sort_by{|r, _| r}.each do |resource, schema|
    puts "## %s" % resource
    puts
    puts "### Attribute Reference"
    puts

    schema["block"]["attributes"].each do |attribute, aschema|
        next if aschema["computed"]

        description = "%s (%s)" % [aschema["description"], aschema["type"]]
        if aschema["optional"]
            description = "(optional) %s" % description
        end

        puts " * `%s` - %s" % [attribute, description]
    end

    puts
end
