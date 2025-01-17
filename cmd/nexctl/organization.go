package main

import (
	"context"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v3"
)

func createOrganizationCommand() *cli.Command {
	return &cli.Command{
		Name:  "organization",
		Usage: "Commands relating to organizations",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List organizations",
				Action: func(ctx context.Context, command *cli.Command) error {
					return listOrganizations(ctx, command)
				},
			},
			{
				Name:  "create",
				Usage: "Create a organizations",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: true,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					name := command.String("name")
					description := command.String("description")
					return createOrganization(ctx, command, name, description)
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a organization",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "organization-id",
						Required: true,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					organizationID, err := getUUID(command, "organization-id")
					if err != nil {
						return err
					}

					return deleteOrganization(ctx, command, organizationID)
				},
			},
		},
	}
}

func orgTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "ORGANIZATION ID", Field: "Id"})
	fields = append(fields, TableField{Header: "NAME", Field: "Name"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	return fields
}
func listOrganizations(ctx context.Context, command *cli.Command) error {
	c := createClient(ctx, command)
	res := apiResponse(c.OrganizationsApi.
		ListOrganizations(ctx).
		Execute())
	show(command, orgTableFields(), res)
	return nil
}

func createOrganization(ctx context.Context, command *cli.Command, name, description string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.OrganizationsApi.
		CreateOrganization(ctx).
		Organization(public.ModelsAddOrganization{
			Name:        name,
			Description: description,
		}).Execute())
	show(command, orgTableFields(), res)
	return nil
}

/*
func moveUserToOrganization(c *client.APIClient, encodeOut, username, OrganizationID string) error {
	OrganizationUUID, err := uuid.Parse(OrganizationID)
	if err != nil {
		Fatalf("failed to parse a valid UUID from %s %v", OrganizationID, err)
	}

	res, err := c.MoveCurrentUserToOrganization(OrganizationUUID)
	if err != nil {
		Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("%s successfully moved into Organization %s\n", username, OrganizationID)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		Fatalf("failed to print output: %v", err)
	}

	return nil
}
*/

func deleteOrganization(ctx context.Context, command *cli.Command, id string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.OrganizationsApi.
		DeleteOrganization(ctx, id).
		Execute())
	show(command, orgTableFields(), res)
	showSuccessfully(command, "deleted")
	return nil
}
