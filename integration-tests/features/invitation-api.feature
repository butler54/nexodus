Feature: Invitations API
  Background:
    Given a user named "Johnson" with password "testpass"
    Given a user named "Thompson" with password "testpass"
    Given a user named "EvilBob" with password "testpass"

  Scenario: Invite an existing user to an organization by email

    Given I am logged in as "Johnson"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${johnson_user_id}

    Given I am logged in as "Thompson"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${thompson_user_id}
    Given I store the ".username" selection from the response as ${thompson_username}

    # Workaround: we can't seem to create a test user with an email, so set the email with SQL
    When I run SQL "INSERT INTO user_identities (kind, value, user_id) VALUES ('email', '${johnson_user_id}@redhat.com', '${johnson_user_id}')" expect 1 row to be affected.

    #
    # Verify Thompson can invite Johnson to his org:
    When I POST path "/api/invitations" with json body:
      """
      {
        "email": "${johnson_user_id}@redhat.com",
        "organization_id": "${thompson_user_id}"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${invitation_id}
    And the response should match json:
      """
      {
        "expires_at": "${response.expires_at}",
        "id": "${invitation_id}",
        "organization_id": "${thompson_user_id}",
        "email": "${johnson_user_id}@redhat.com",
        "user_id": "${johnson_user_id}"
      }
      """

  Scenario: Invite a user to an organization by user id

    #
    # Get the user and default org ids for two users...
    #
    Given I am logged in as "Johnson"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${johnson_user_id}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${johnson_organization_id}
    Given I store the ${response[0]} as ${johnson_organization}


    Given I am logged in as "Thompson"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${thompson_user_id}
    Given I store the ".username" selection from the response as ${thompson_username}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${thompson_organization_id}
    Given I store the ${response[0]} as ${thompson_organization}


    # Current user is Thompson.. try to self add to Johnson's org.
    # this should not be allowed.
    When I POST path "/api/invitations" with json body:
      """
      {
        "user_id": "${thompson_user_id}",
        "organization_id": "${johnson_organization_id}"
      }
      """
    Then the response code should be 404
    And the response should match json:
      """
      {"error":"not found","resource":"organization"}
      """

    #
    # Verify Thompson can invite Johnson to his org:
    When I POST path "/api/invitations" with json body:
      """
      {
        "user_id": "${johnson_user_id}",
        "organization_id": "${thompson_organization_id}"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${invitation_id}
    And the response should match json:
      """
      {
        "expires_at": "${response.expires_at}",
        "id": "${invitation_id}",
        "organization_id": "${thompson_organization_id}",
        "user_id": "${johnson_user_id}"
      }
      """

    #
    # Verify Thompson and Johnson can see the invitation
    When I GET path "/api/invitations"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "expires_at": "${response[0].expires_at}",
          "id": "${invitation_id}",
          "organization_id": "${thompson_organization_id}",
          "from": {
            "full_name": "Test Thompson",
            "id": "${thompson_user_id}",
            "picture": "",
            "username": "${thompson_username}"
          },
          "organization": {
            "description": "${thompson_username}'s organization",
            "id": "${thompson_organization_id}",
            "name": "${thompson_username}",
            "owner_id": "${thompson_user_id}"
          },
          "user_id": "${johnson_user_id}"
        }
      ]
      """

    Given I am logged in as "Johnson"
    When I GET path "/api/invitations"
    Then the response code should be 200
    And the response should match json:
      """
      [
        {
          "expires_at": "${response[0].expires_at}",
          "id": "${invitation_id}",
          "organization_id": "${thompson_organization_id}",
          "from": {
            "full_name": "Test Thompson",
            "id": "${thompson_user_id}",
            "picture": "",
            "username": "${thompson_username}"
          },
          "organization": {
            "description": "${thompson_username}'s organization",
            "id": "${thompson_organization_id}",
            "name": "${thompson_username}",
            "owner_id": "${thompson_user_id}"
          },
          "user_id": "${johnson_user_id}"
        }
      ]
      """

    # But EvilBob should not see the invitation.
    Given I am logged in as "EvilBob"
    When I GET path "/api/invitations"
    Then the response code should be 200
    And the response should match json:
      """
      []
      """

    # Others cannot accept the invitation.
    Given I am logged in as "EvilBob"
    When I POST path "/api/invitations/${invitation_id}/accept"
    Then the response code should be 404
    And the response should match json:
      """
      {"error":"not found","resource":"invitation"}
      """

    Given I am logged in as "Thompson"
    When I POST path "/api/invitations/${invitation_id}/accept"
    Then the response code should be 404
    And the response should match json:
      """
      {"error":"not found","resource":"invitation"}
      """

    # Only Johnson should be able to accept the invitation.
    Given I am logged in as "Johnson"
    When I POST path "/api/invitations/${invitation_id}/accept"
    Then the response code should be 204
    And the response should match ""

    # The invitation should be now deleted...
    When I GET path "/api/invitations"
    Then the response code should be 200
    And the response should match json:
      """
      []
      """

    # Johnson should be in two orgs now...
    When I GET path "/api/organizations"
    Then the response code should be 200
    And the response should match json:
      """
      [ ${johnson_organization}, ${thompson_organization} ]
      """

  Scenario: Receiver of invitation can delete the invitation

    Given I am logged in as "Johnson"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${johnson_user_id}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${johnson_organization_id}

    Given I am logged in as "Thompson"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${thompson_user_id}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${thompson_organization_id}


    # Create the invite.
    When I POST path "/api/invitations" with json body:
      """
      {
        "user_id": "${johnson_user_id}",
        "organization_id": "${thompson_organization_id}"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${invitation_id}

    # EvilBob cannot delete the invitation.
    Given I am logged in as "EvilBob"
    When I DELETE path "/api/invitations/${invitation_id}"
    Then the response code should be 404
    And the response should match json:
      """
      {"error":"not found","resource":"invitation"}
      """

    Given I am logged in as "Johnson"
    When I DELETE path "/api/invitations/${invitation_id}"
    Then the response code should be 204
    And the response should match ""

  Scenario: Sender of invitation can delete the invitation

    Given I am logged in as "Johnson"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${johnson_user_id}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${johnson_organization_id}

    Given I am logged in as "Thompson"
    When I GET path "/api/users/me"
    Then the response code should be 200
    Given I store the ".id" selection from the response as ${thompson_user_id}

    When I GET path "/api/organizations"
    Then the response code should be 200
    Given I store the ${response[0].id} as ${thompson_organization_id}

    # Create the invite.
    When I POST path "/api/invitations" with json body:
      """
      {
        "user_id": "${johnson_user_id}",
        "organization_id": "${thompson_organization_id}"
      }
      """
    Then the response code should be 201
    Given I store the ".id" selection from the response as ${invitation_id}

    Given I am logged in as "Thompson"
    When I DELETE path "/api/invitations/${invitation_id}"
    Then the response code should be 204
    And the response should match ""
