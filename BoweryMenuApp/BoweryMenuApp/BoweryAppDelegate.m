//
//  BoweryAppDelegate.m
//  BoweryMenuApp
//
//  Created by Steve Kaliski on 7/10/14.
//  Copyright (c) 2014 Bowery. All rights reserved.
//

#import "BoweryAppDelegate.h"
#import <WebKit/WebKit.h>

@implementation BoweryAppDelegate

@synthesize refreshBtn = _refreshBtn;

- (void)applicationDidFinishLaunching:(NSNotification *)aNotification
{
	NSURLRequest *request = [NSURLRequest requestWithURL:[NSURL URLWithString:@"http://localhost:32055/"]];
	[self.webView.mainFrame loadRequest:request];
}

- (IBAction)doSomething:(id)sender {
    
    // Load Google and change button accordingly
    if ([[_refreshBtn title] isEqualTo:@"Load Yahoo"]) {
        [_refreshBtn setTitle:@"Load Google"];
        NSURLRequest *request = [NSURLRequest requestWithURL:[NSURL URLWithString:@"http://yahoo.com"]];
        [self.webView.mainFrame loadRequest:request];

    }
    
    // Load Yahoo and change button accordingly
    else {
        [_refreshBtn setTitle:@"Load Yahoo"];
        NSURLRequest *request = [NSURLRequest requestWithURL:[NSURL URLWithString:@"http://google.com"]];
        [self.webView.mainFrame loadRequest:request];
        
    }

}

@end
